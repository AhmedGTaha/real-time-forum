package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const defaultMessageLimit = 10
const maxMessageLimit = 50

// chatUserResponse is the JSON shape sent to the frontend for each user in
// the chat sidebar/list.
type chatUserResponse struct {
	ID            int    `json:"id"`
	Nickname      string `json:"nickname"`
	Online        bool   `json:"online"`
	LastMessageAt string `json:"last_message_at"`
}

type chatUsersResponse struct {
	Users []chatUserResponse `json:"users"`
}

// chatMessageResponse is one message as the frontend expects to receive it.
// It includes both IDs for logic and the sender nickname for display.
type chatMessageResponse struct {
	ID             int    `json:"id"`
	SenderID       int    `json:"sender_id"`
	SenderNickname string `json:"sender_nickname"`
	ReceiverID     int    `json:"receiver_id"`
	Content        string `json:"content"`
	CreatedAt      string `json:"created_at"`
}

type chatMessagesResponse struct {
	Messages []chatMessageResponse `json:"messages"`
}

func (app *App) ChatUsersHandler(w http.ResponseWriter, r *http.Request) {
	// This endpoint only reads data, so reject anything except GET.
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// All chat endpoints need a logged-in user because chats are private.
	currentUser, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	// Load every other user, plus the newest message time between that user and
	// the current user. LEFT JOIN keeps users in the list even when no messages
	// have been exchanged yet.
	rows, err := app.DB.Query(`
		SELECT
			users.id,
			users.nickname,
			COALESCE(MAX(messages.created_at), '') AS last_message_at
		FROM users
		LEFT JOIN messages
			ON (
				(messages.sender_id = ? AND messages.receiver_id = users.id)
				OR
				(messages.sender_id = users.id AND messages.receiver_id = ?)
			)
		WHERE users.id != ?
		GROUP BY users.id
		ORDER BY
			CASE
				-- Put users with no conversation after users with messages.
				WHEN MAX(messages.created_at) IS NULL THEN 1
				ELSE 0
			END,
			-- Most recent conversations appear first, then names alphabetically.
			MAX(messages.created_at) DESC,
			users.nickname COLLATE NOCASE ASC;
	`, currentUser.ID, currentUser.ID, currentUser.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load chat users")
		return
	}
	defer rows.Close()

	users := []chatUserResponse{}

	for rows.Next() {
		var user chatUserResponse

		err := rows.Scan(
			&user.ID,
			&user.Nickname,
			&user.LastMessageAt,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read chat user")
			return
		}

		// Online status is part of the response already, but real presence is not
		// implemented yet. For now every user is reported as offline.
		user.Online = app.Hub.IsOnline(user.ID)
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to finish reading chat users")
		return
	}

	writeJSON(w, http.StatusOK, chatUsersResponse{
		Users: users,
	})
}

func (app *App) ChatMessagesHandler(w http.ResponseWriter, r *http.Request) {
	// Message history is read-only from this endpoint.
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	currentUser, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	// user_id tells us whose conversation the current user wants to open.
	otherUserID, err := readPositiveIntQuery(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "user_id must be a valid number")
		return
	}

	// A private chat needs two different users.
	if otherUserID == currentUser.ID {
		writeError(w, http.StatusBadRequest, "cannot load chat with yourself")
		return
	}

	// Give a clear 404 if the frontend asks for a user that does not exist.
	exists, err := app.userExists(otherUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check user")
		return
	}

	if !exists {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// limit controls page size. before_id is used for "load older messages":
	// asking for messages before ID 120 returns messages with IDs lower than 120.
	limit := readOptionalLimitQuery(r)
	beforeID := readOptionalBeforeIDQuery(r)

	// The inner query grabs the newest matching messages first so LIMIT gets the
	// latest page. The outer query flips them back to oldest-to-newest so the
	// frontend can render the conversation in normal reading order.
	rows, err := app.DB.Query(`
		SELECT
			id,
			sender_id,
			sender_nickname,
			receiver_id,
			content,
			created_at
		FROM (
			SELECT
				messages.id,
				messages.sender_id,
				sender.nickname AS sender_nickname,
				messages.receiver_id,
				messages.content,
				messages.created_at
			FROM messages
			INNER JOIN users AS sender ON sender.id = messages.sender_id
			WHERE
				(
					(messages.sender_id = ? AND messages.receiver_id = ?)
					OR
					(messages.sender_id = ? AND messages.receiver_id = ?)
				)
				-- before_id is optional. When it is 0, this condition allows all rows.
				AND (? = 0 OR messages.id < ?)
			ORDER BY messages.id DESC
			LIMIT ?
		)
		ORDER BY id ASC;
	`,
		currentUser.ID,
		otherUserID,
		otherUserID,
		currentUser.ID,
		beforeID,
		beforeID,
		limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load messages")
		return
	}
	defer rows.Close()

	messages := []chatMessageResponse{}

	for rows.Next() {
		var message chatMessageResponse

		err := rows.Scan(
			&message.ID,
			&message.SenderID,
			&message.SenderNickname,
			&message.ReceiverID,
			&message.Content,
			&message.CreatedAt,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read message")
			return
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to finish reading messages")
		return
	}

	writeJSON(w, http.StatusOK, chatMessagesResponse{
		Messages: messages,
	})
}

// readPositiveIntQuery reads a required query string value like ?user_id=3.
func readPositiveIntQuery(r *http.Request, key string) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return 0, errors.New("missing query value")
	}

	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 {
		return 0, errors.New("invalid query value")
	}

	return number, nil
}

// readOptionalLimitQuery returns a safe page size for loading messages.
// Bad or missing values fall back to the default, and very large values are capped.
func readOptionalLimitQuery(r *http.Request) int {
	value := strings.TrimSpace(r.URL.Query().Get("limit"))
	if value == "" {
		return defaultMessageLimit
	}

	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return defaultMessageLimit
	}

	if limit > maxMessageLimit {
		return maxMessageLimit
	}

	return limit
}

// readOptionalBeforeIDQuery reads ?before_id=123 for pagination.
// Returning 0 means "no before_id filter".
func readOptionalBeforeIDQuery(r *http.Request) int {
	value := strings.TrimSpace(r.URL.Query().Get("before_id"))
	if value == "" {
		return 0
	}

	beforeID, err := strconv.Atoi(value)
	if err != nil || beforeID <= 0 {
		return 0
	}

	return beforeID
}

// userExists checks whether a user ID is real without loading the full user row.
func (app *App) userExists(userID int) (bool, error) {
	var id int

	err := app.DB.QueryRow(`
		SELECT id
		FROM users
		WHERE id = ?
		LIMIT 1;
	`, userID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

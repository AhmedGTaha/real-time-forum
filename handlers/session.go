package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
)

type currentUser struct {
	ID        int    `json:"id"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// meResponse is the JSON shape the browser receives when it asks for the current user.
type meResponse struct {
	User currentUser `json:"user"`
}

func (app *App) CurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	writeJSON(w, http.StatusOK, meResponse{
		User: user,
	})
}

// returns the current user based on the session cookie in the request.
func (app *App) GetCurrentUser(r *http.Request) (currentUser, error) {
	// Look for the session cookie in the request. If it's not there, return an error.
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return currentUser{}, err
	}

	// Trim any whitespace from the cookie value. If it's empty, return an error.
	sessionID := strings.TrimSpace(cookie.Value)
	if sessionID == "" {
		return currentUser{}, errors.New("missing session id")
	}

	var user currentUser
	var expiresAt time.Time

	// This finds the session and the user connected to it.
	err = app.DB.QueryRow(`
		SELECT 
			users.id,
			users.nickname,
			users.email,
			users.first_name,
			users.last_name,
			sessions.expires_at
		FROM sessions
		INNER JOIN users ON users.id = sessions.user_id
		WHERE sessions.id = ?
		LIMIT 1;
	`, sessionID).Scan(
		&user.ID,
		&user.Nickname,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&expiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return currentUser{}, errors.New("session not found")
		}

		return currentUser{}, err
	}

	if time.Now().UTC().After(expiresAt) {
		_, _ = app.DB.Exec(`
			DELETE FROM sessions
			WHERE id = ?;
		`, sessionID)

		return currentUser{}, errors.New("session expired")
	}

	return user, nil
}
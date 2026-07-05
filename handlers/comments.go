package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type createCommentRequest struct {
	// These JSON tags must match the keys sent by the frontend.
	// Example: {"post_id":1,"content":"Nice post!"}
	PostID  int    `json:"post_id"`
	Content string `json:"content"`
}

type commentResponse struct {
	// This is the JSON shape used when sending one comment back to the browser.
	ID        int    `json:"id"`
	PostID    int    `json:"post_id"`
	AuthorID  int    `json:"author_id"`
	Author    string `json:"author"`
	Content   string `json:"content"`
	LikeCount int    `json:"like_count"`
	CreatedAt string `json:"created_at"`
}

type commentsResponse struct {
	// Wrapping the slice gives the response a stable shape: {"comments":[...]}.
	Comments []commentResponse `json:"comments"`
}

func (app *App) CommentsHandler(w http.ResponseWriter, r *http.Request) {
	// One route can do different things depending on the HTTP method.
	switch r.Method {
	case http.MethodGet:
		app.ListCommentsHandler(w, r)
	case http.MethodPost:
		app.CreateCommentHandler(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (app *App) CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
	// Creating a comment requires a logged-in user, because every comment needs an author.
	user, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	var req createCommentRequest

	// readJSON turns the browser's JSON body into the Go struct above.
	// The & means "pass the address", so readJSON can fill req.
	err = readJSON(w, r, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Clean first, then validate. That way spaces-only content becomes empty.
	req.clean()

	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Make sure the comment is attached to a real post before inserting it.
	exists, err := app.postExists(req.PostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check post")
		return
	}

	if !exists {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}

	// Insert the comment using the post id from JSON and the user id from the session.
	result, err := app.DB.Exec(`
		INSERT INTO comments (post_id, user_id, content)
		VALUES (?, ?, ?);
	`, req.PostID, user.ID, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create comment")
		return
	}

	// LastInsertId reads the id SQLite created for the new comment.
	commentID, err := result.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read comment id")
		return
	}

	// Send JSON back so the frontend knows the comment was created.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      commentID,
		"message": "comment created successfully",
	})
}

func (app *App) ListCommentsHandler(w http.ResponseWriter, r *http.Request) {
	// Only logged-in users can load comments.
	_, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	// GET requests do not have a JSON body here, so post_id comes from the URL:
	// /api/comments?post_id=1
	postIDText := strings.TrimSpace(r.URL.Query().Get("post_id"))
	if postIDText == "" {
		writeError(w, http.StatusBadRequest, "post_id is required")
		return
	}

	// Query values are strings, so convert post_id into an int before using it.
	postID, err := strconv.Atoi(postIDText)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusBadRequest, "post_id must be a valid number")
		return
	}

	// Return 404 if the frontend asks for comments on a post that does not exist.
	exists, err := app.postExists(postID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check post")
		return
	}

	if !exists {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}

	// Load comments for one post with each author's nickname and like count.
	rows, err := app.DB.Query(`
		SELECT
			comments.id,
			comments.post_id,
			users.id,
			users.nickname,
			comments.content,
			COUNT(DISTINCT comment_likes.user_id),
			comments.created_at
		FROM comments
		INNER JOIN users ON users.id = comments.user_id
		LEFT JOIN comment_likes ON comment_likes.comment_id = comments.id
		WHERE comments.post_id = ?
		GROUP BY comments.id
		ORDER BY comments.created_at ASC;
	`, postID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load comments")
		return
	}
	defer rows.Close()

	// Start with an empty slice so JSON returns [] instead of null.
	comments := []commentResponse{}

	for rows.Next() {
		var comment commentResponse

		// Scan copies the current database row into the comment struct.
		err := rows.Scan(
			&comment.ID,
			&comment.PostID,
			&comment.AuthorID,
			&comment.Author,
			&comment.Content,
			&comment.LikeCount,
			&comment.CreatedAt,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read comment")
			return
		}

		comments = append(comments, comment)
	}

	// rows.Err catches errors that can happen during the loop.
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to finish reading comments")
		return
	}

	// writeJSON converts commentsResponse into {"comments":[...]}.
	writeJSON(w, http.StatusOK, commentsResponse{
		Comments: comments,
	})
}

func (req *createCommentRequest) clean() {
	// Pointer receiver (*) lets this method update the original request struct.
	req.Content = strings.TrimSpace(req.Content)
}

func (req createCommentRequest) validate() error {
	// These errors are sent back to the frontend as JSON error messages.
	if req.PostID <= 0 {
		return errors.New("post_id is required")
	}

	if req.Content == "" {
		return errors.New("content is required")
	}

	return nil
}

func (app *App) postExists(postID int) (bool, error) {
	// We only need to know whether a row exists, so selecting the id is enough.
	var id int

	err := app.DB.QueryRow(`
		SELECT id
		FROM posts
		WHERE id = ?
		LIMIT 1;
	`, postID).Scan(&id)
	if err != nil {
		// sql.ErrNoRows means the query worked, but no post matched this id.
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		// Any other error is a real database problem.
		return false, err
	}

	// If Scan succeeded, the post exists.
	return true, nil
}

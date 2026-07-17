package handlers

import (
	"net/http"
)

// requireAdmin returns the current user only when they are logged in AND flagged
// as an admin. Handlers use it to gate every admin-only action.
func (app *App) requireAdmin(w http.ResponseWriter, r *http.Request) (currentUser, bool) {
	user, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "please log in to continue")
		return currentUser{}, false
	}

	if !user.IsAdmin {
		writeError(w, http.StatusForbidden, "admin access is required")
		return currentUser{}, false
	}

	return user, true
}

// --- overview -------------------------------------------------------------

type adminUserRow struct {
	ID           int    `json:"id"`
	Nickname     string `json:"nickname"`
	Email        string `json:"email"`
	IsAdmin      bool   `json:"is_admin"`
	PostCount    int    `json:"post_count"`
	CommentCount int    `json:"comment_count"`
}

type adminPostRow struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	AuthorID     int    `json:"author_id"`
	LikeCount    int    `json:"like_count"`
	CommentCount int    `json:"comment_count"`
	CreatedAt    string `json:"created_at"`
}

type adminCommentRow struct {
	ID        int    `json:"id"`
	Content   string `json:"content"`
	Author    string `json:"author"`
	AuthorID  int    `json:"author_id"`
	PostID    int    `json:"post_id"`
	PostTitle string `json:"post_title"`
	CreatedAt string `json:"created_at"`
}

type adminOverviewResponse struct {
	Users    []adminUserRow    `json:"users"`
	Posts    []adminPostRow    `json:"posts"`
	Comments []adminCommentRow `json:"comments"`
}

// AdminOverviewHandler returns every user, post, and comment so the admin page
// can list them with delete controls.
func (app *App) AdminOverviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "use GET to load the admin overview")
		return
	}

	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}

	users, err := app.adminListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load users")
		return
	}

	posts, err := app.adminListPosts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load posts")
		return
	}

	comments, err := app.adminListComments()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load comments")
		return
	}

	writeJSON(w, http.StatusOK, adminOverviewResponse{
		Users:    users,
		Posts:    posts,
		Comments: comments,
	})
}

func (app *App) adminListUsers() ([]adminUserRow, error) {
	rows, err := app.DB.Query(`
		SELECT
			users.id,
			users.nickname,
			users.email,
			users.is_admin,
			(SELECT COUNT(*) FROM posts WHERE posts.user_id = users.id),
			(SELECT COUNT(*) FROM comments WHERE comments.user_id = users.id)
		FROM users
		ORDER BY users.id ASC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []adminUserRow{}
	for rows.Next() {
		var u adminUserRow
		if err := rows.Scan(&u.ID, &u.Nickname, &u.Email, &u.IsAdmin, &u.PostCount, &u.CommentCount); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func (app *App) adminListPosts() ([]adminPostRow, error) {
	rows, err := app.DB.Query(`
		SELECT
			posts.id,
			posts.title,
			users.nickname,
			users.id,
			posts.created_at,
			(SELECT COUNT(*) FROM post_likes WHERE post_likes.post_id = posts.id),
			(SELECT COUNT(*) FROM comments WHERE comments.post_id = posts.id)
		FROM posts
		INNER JOIN users ON users.id = posts.user_id
		ORDER BY posts.created_at DESC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []adminPostRow{}
	for rows.Next() {
		var p adminPostRow
		if err := rows.Scan(&p.ID, &p.Title, &p.Author, &p.AuthorID, &p.CreatedAt, &p.LikeCount, &p.CommentCount); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}

	return posts, rows.Err()
}

func (app *App) adminListComments() ([]adminCommentRow, error) {
	rows, err := app.DB.Query(`
		SELECT
			comments.id,
			comments.content,
			users.nickname,
			users.id,
			comments.post_id,
			posts.title,
			comments.created_at
		FROM comments
		INNER JOIN users ON users.id = comments.user_id
		INNER JOIN posts ON posts.id = comments.post_id
		ORDER BY comments.created_at DESC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := []adminCommentRow{}
	for rows.Next() {
		var c adminCommentRow
		if err := rows.Scan(&c.ID, &c.Content, &c.Author, &c.AuthorID, &c.PostID, &c.PostTitle, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}

	return comments, rows.Err()
}

// --- deletes --------------------------------------------------------------

type adminDeletePostRequest struct {
	PostID int `json:"post_id"`
}

type adminDeleteCommentRequest struct {
	CommentID int `json:"comment_id"`
}

type adminDeleteUserRequest struct {
	UserID int `json:"user_id"`
}

// AdminPostsHandler lets an admin delete any post (not just their own).
func (app *App) AdminPostsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", http.MethodDelete)
		writeError(w, http.StatusMethodNotAllowed, "use DELETE to remove a post")
		return
	}

	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}

	var req adminDeletePostRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON with post_id")
		return
	}

	if req.PostID <= 0 {
		writeError(w, http.StatusBadRequest, "post_id must be a positive number")
		return
	}

	tx, err := app.DB.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start deleting the post")
		return
	}

	if err := deletePostAndChildren(tx, req.PostID); err != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "could not delete the post")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not finish deleting the post")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "post deleted successfully"})
}

// AdminCommentsHandler lets an admin delete any comment.
func (app *App) AdminCommentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", http.MethodDelete)
		writeError(w, http.StatusMethodNotAllowed, "use DELETE to remove a comment")
		return
	}

	if _, ok := app.requireAdmin(w, r); !ok {
		return
	}

	var req adminDeleteCommentRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON with comment_id")
		return
	}

	if req.CommentID <= 0 {
		writeError(w, http.StatusBadRequest, "comment_id must be a positive number")
		return
	}

	tx, err := app.DB.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start deleting the comment")
		return
	}

	if err := deleteCommentAndChildren(tx, req.CommentID); err != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "could not delete the comment")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not finish deleting the comment")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "comment deleted successfully"})
}

// AdminUsersHandler lets an admin delete a user account. Everything the user
// produced is removed too: their posts (and all likes/comments on those posts),
// their comments (and likes on them), their own likes, and their messages.
func (app *App) AdminUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", http.MethodDelete)
		writeError(w, http.StatusMethodNotAllowed, "use DELETE to remove an account")
		return
	}

	admin, ok := app.requireAdmin(w, r)
	if !ok {
		return
	}

	var req adminDeleteUserRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON with user_id")
		return
	}

	if req.UserID <= 0 {
		writeError(w, http.StatusBadRequest, "user_id must be a positive number")
		return
	}

	// Guard against an admin deleting the account they are logged in with.
	if req.UserID == admin.ID {
		writeError(w, http.StatusBadRequest, "you cannot delete your own admin account")
		return
	}

	exists, err := app.userExists(req.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not check the account")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}

	if err := app.deleteUserAndContent(req.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete the account")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "account deleted successfully"})
}

// deleteUserAndContent removes a user and every row connected to them, deleting
// child rows before parents so nothing is left dangling.
func (app *App) deleteUserAndContent(userID int) error {
	tx, err := app.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Ordered so that dependent rows go before the rows they point at.
	steps := []struct {
		query string
		args  []any
	}{
		// Comment likes: those the user made, those on the user's comments, and
		// those on comments attached to the user's posts.
		{`DELETE FROM comment_likes WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM comment_likes WHERE comment_id IN (SELECT id FROM comments WHERE user_id = ?);`, []any{userID}},
		{`DELETE FROM comment_likes WHERE comment_id IN (SELECT id FROM comments WHERE post_id IN (SELECT id FROM posts WHERE user_id = ?));`, []any{userID}},

		// Comments: written by the user, or left on the user's posts.
		{`DELETE FROM comments WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM comments WHERE post_id IN (SELECT id FROM posts WHERE user_id = ?);`, []any{userID}},

		// Post likes: made by the user, or on the user's posts.
		{`DELETE FROM post_likes WHERE user_id = ?;`, []any{userID}},
		{`DELETE FROM post_likes WHERE post_id IN (SELECT id FROM posts WHERE user_id = ?);`, []any{userID}},

		// Category links and the posts themselves.
		{`DELETE FROM post_categories WHERE post_id IN (SELECT id FROM posts WHERE user_id = ?);`, []any{userID}},
		{`DELETE FROM posts WHERE user_id = ?;`, []any{userID}},

		// Private messages and sessions belonging to the user.
		{`DELETE FROM messages WHERE sender_id = ? OR receiver_id = ?;`, []any{userID, userID}},
		{`DELETE FROM sessions WHERE user_id = ?;`, []any{userID}},

		// Finally the account row.
		{`DELETE FROM users WHERE id = ?;`, []any{userID}},
	}

	for _, step := range steps {
		if _, err := tx.Exec(step.query, step.args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

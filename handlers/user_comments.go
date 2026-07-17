package handlers

import (
	"database/sql"
	"net/http"
)

// userCommentResponse is a comment shown in the sidebar "Liked comments" and
// "My comments" views. Unlike commentResponse it also carries the parent post
// title so the frontend can show which post the comment belongs to.
type userCommentResponse struct {
	ID        int    `json:"id"`
	PostID    int    `json:"post_id"`
	PostTitle string `json:"post_title"`
	AuthorID  int    `json:"author_id"`
	Author    string `json:"author"`
	Content   string `json:"content"`
	LikeCount int    `json:"like_count"`
	Liked     bool   `json:"liked"`
	CreatedAt string `json:"created_at"`
}

type userCommentsResponse struct {
	Comments []userCommentResponse `json:"comments"`
}

// LikedCommentsHandler returns the comments the current user has liked.
func (app *App) LikedCommentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "use GET to list liked comments")
		return
	}

	user, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "please log in to view liked comments")
		return
	}

	// The INNER JOIN on liked_by_me keeps only comments this user liked, while the
	// separate LEFT JOIN counts likes from everyone.
	rows, err := app.DB.Query(`
		SELECT
			comments.id,
			comments.post_id,
			posts.title,
			author.id,
			author.nickname,
			comments.content,
			COUNT(DISTINCT comment_likes.user_id),
			COALESCE(MAX(CASE WHEN comment_likes.user_id = ? THEN 1 ELSE 0 END), 0),
			comments.created_at
		FROM comments
		INNER JOIN comment_likes AS liked_by_me
			ON liked_by_me.comment_id = comments.id AND liked_by_me.user_id = ?
		INNER JOIN users AS author ON author.id = comments.user_id
		INNER JOIN posts ON posts.id = comments.post_id
		LEFT JOIN comment_likes ON comment_likes.comment_id = comments.id
		GROUP BY comments.id
		ORDER BY comments.created_at DESC
		LIMIT 50;
	`, user.ID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load liked comments right now")
		return
	}
	defer rows.Close()

	comments, err := scanUserComments(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read liked comments")
		return
	}

	writeJSON(w, http.StatusOK, userCommentsResponse{Comments: comments})
}

// MyCommentsHandler returns the comments the current user has written.
func (app *App) MyCommentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "use GET to list your comments")
		return
	}

	user, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "please log in to view your comments")
		return
	}

	rows, err := app.DB.Query(`
		SELECT
			comments.id,
			comments.post_id,
			posts.title,
			author.id,
			author.nickname,
			comments.content,
			COUNT(DISTINCT comment_likes.user_id),
			COALESCE(MAX(CASE WHEN comment_likes.user_id = ? THEN 1 ELSE 0 END), 0),
			comments.created_at
		FROM comments
		INNER JOIN users AS author ON author.id = comments.user_id
		INNER JOIN posts ON posts.id = comments.post_id
		LEFT JOIN comment_likes ON comment_likes.comment_id = comments.id
		WHERE comments.user_id = ?
		GROUP BY comments.id
		ORDER BY comments.created_at DESC
		LIMIT 50;
	`, user.ID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load your comments right now")
		return
	}
	defer rows.Close()

	comments, err := scanUserComments(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read your comments")
		return
	}

	writeJSON(w, http.StatusOK, userCommentsResponse{Comments: comments})
}

// scanUserComments reads the shared column layout used by both comment views.
func scanUserComments(rows *sql.Rows) ([]userCommentResponse, error) {
	comments := []userCommentResponse{}

	for rows.Next() {
		var comment userCommentResponse

		err := rows.Scan(
			&comment.ID,
			&comment.PostID,
			&comment.PostTitle,
			&comment.AuthorID,
			&comment.Author,
			&comment.Content,
			&comment.LikeCount,
			&comment.Liked,
			&comment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return comments, nil
}

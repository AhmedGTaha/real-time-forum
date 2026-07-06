package handlers

import (
	"database/sql"
	"errors"
	"net/http"
)

type togglePostLikeRequest struct {
	PostID int `json:"post_id"`
}

type toggleCommentLikeRequest struct {
	CommentID int `json:"comment_id"`
}

type likeResponse struct {
	Liked     bool `json:"liked"`
	LikeCount int  `json:"like_count"`
}

func (app *App) TogglePostLikeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	var req togglePostLikeRequest

	err = readJSON(w, r, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.PostID <= 0 {
		writeError(w, http.StatusBadRequest, "post_id is required")
		return
	}

	exists, err := app.postExists(req.PostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check post")
		return
	}

	if !exists {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}

	liked, err := app.togglePostLike(req.PostID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to toggle post like")
		return
	}

	likeCount, err := app.countPostLikes(req.PostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count post likes")
		return
	}

	writeJSON(w, http.StatusOK, likeResponse{
		Liked:     liked,
		LikeCount: likeCount,
	})
}

func (app *App) ToggleCommentLikeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	var req toggleCommentLikeRequest

	err = readJSON(w, r, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.CommentID <= 0 {
		writeError(w, http.StatusBadRequest, "comment_id is required")
		return
	}

	exists, err := app.commentExists(req.CommentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check comment")
		return
	}

	if !exists {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}

	liked, err := app.toggleCommentLike(req.CommentID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to toggle comment like")
		return
	}

	likeCount, err := app.countCommentLikes(req.CommentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count comment likes")
		return
	}

	writeJSON(w, http.StatusOK, likeResponse{
		Liked:     liked,
		LikeCount: likeCount,
	})
}

func (app *App) togglePostLike(postID int, userID int) (bool, error) {
	var existingUserID int

	err := app.DB.QueryRow(`
		SELECT user_id
		FROM post_likes
		WHERE post_id = ? AND user_id = ?
		LIMIT 1;
	`, postID, userID).Scan(&existingUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, err = app.DB.Exec(`
				INSERT INTO post_likes (post_id, user_id)
				VALUES (?, ?);
			`, postID, userID)

			if err != nil {
				return false, err
			}

			return true, nil
		}

		return false, err
	}

	_, err = app.DB.Exec(`
		DELETE FROM post_likes
		WHERE post_id = ? AND user_id = ?;
	`, postID, userID)
	if err != nil {
		return false, err
	}

	return false, nil
}

func (app *App) toggleCommentLike(commentID int, userID int) (bool, error) {
	var existingUserID int

	err := app.DB.QueryRow(`
		SELECT user_id
		FROM comment_likes
		WHERE comment_id = ? AND user_id = ?
		LIMIT 1;
	`, commentID, userID).Scan(&existingUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_, err = app.DB.Exec(`
				INSERT INTO comment_likes (comment_id, user_id)
				VALUES (?, ?);
			`, commentID, userID)

			if err != nil {
				return false, err
			}

			return true, nil
		}

		return false, err
	}

	_, err = app.DB.Exec(`
		DELETE FROM comment_likes
		WHERE comment_id = ? AND user_id = ?;
	`, commentID, userID)
	if err != nil {
		return false, err
	}

	return false, nil
}

func (app *App) countPostLikes(postID int) (int, error) {
	var count int

	err := app.DB.QueryRow(`
		SELECT COUNT(*)
		FROM post_likes
		WHERE post_id = ?;
	`, postID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (app *App) countCommentLikes(commentID int) (int, error) {
	var count int

	err := app.DB.QueryRow(`
		SELECT COUNT(*)
		FROM comment_likes
		WHERE comment_id = ?;
	`, commentID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (app *App) commentExists(commentID int) (bool, error) {
	var id int

	err := app.DB.QueryRow(`
		SELECT id
		FROM comments
		WHERE id = ?
		LIMIT 1;
	`, commentID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

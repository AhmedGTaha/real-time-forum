package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

type createPostRequest struct {
	// These JSON tags must match the keys sent by the frontend.
	// Example: {"title":"Hello","content":"...","categories":["go"]}
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Categories []string `json:"categories"`
}

type postResponse struct {
	// This is the JSON shape used when sending one post back to the browser.
	ID           int      `json:"id"`
	Title        string   `json:"title"`
	Content      string   `json:"content"`
	AuthorID     int      `json:"author_id"`
	Author       string   `json:"author"`
	Categories   []string `json:"categories"`
	LikeCount    int      `json:"like_count"`
	CommentCount int      `json:"comment_count"`
	CreatedAt    string   `json:"created_at"`
}

type postsFeedResponse struct {
	// Wrapping the slice gives the response a stable shape: {"posts":[...]}.
	Posts []postResponse `json:"posts"`
}

// PostsHandler chooses the correct action based on request method.
func (app *App) PostsHandler(w http.ResponseWriter, r *http.Request) {
	// One route can do different things depending on the HTTP method.
	switch r.Method {
	case http.MethodGet:
		app.ListPostsHandler(w, r)
	case http.MethodPost:
		app.CreatePostHandler(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (app *App) CreatePostHandler(w http.ResponseWriter, r *http.Request) {
	// Creating a post requires a logged-in user, because every post needs an author.
	user, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	var req createPostRequest

	// readJSON turns the browser's JSON body into the Go struct above.
	err = readJSON(w, r, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.clean()

	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// A transaction keeps the post and category links together.
	// If anything fails, Rollback cancels the whole create-post operation.
	tx, err := app.DB.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}

	// Insert the post first so SQLite can create its id.
	result, err := tx.Exec(`
		INSERT INTO posts (user_id, title, content)
		VALUES (?, ?, ?);
	`, user.ID, req.Title, req.Content)
	if err != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "failed to create post")
		return
	}

	postID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "failed to read post id")
		return
	}

	// Categories live in their own table. post_categories connects each
	// category to this post.
	for _, categoryName := range req.Categories {
		categoryID, err := getOrCreateCategoryID(tx, categoryName)
		if err != nil {
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "failed to create category")
			return
		}

		_, err = tx.Exec(`
			INSERT OR IGNORE INTO post_categories (post_id, category_id)
			VALUES (?, ?);
		`, postID, categoryID)
		if err != nil {
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "failed to connect category to post")
			return
		}
	}

	// Commit makes all changes in the transaction permanent.
	err = tx.Commit()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save post")
		return
	}

	// Send JSON back so the frontend knows the post was created.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      postID,
		"message": "post created successfully",
	})
}

func (app *App) ListPostsHandler(w http.ResponseWriter, r *http.Request) {
	// Only logged-in users can load the posts feed.
	_, err := app.GetCurrentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not logged in")
		return
	}

	// This query loads posts plus their author, categories, likes, and comments.
	rows, err := app.DB.Query(`
		SELECT
			posts.id,
			posts.title,
			posts.content,
			users.id,
			users.nickname,
			COALESCE(GROUP_CONCAT(DISTINCT categories.name), ''),
			COUNT(DISTINCT post_likes.user_id),
			COUNT(DISTINCT comments.id),
			posts.created_at
		FROM posts
		INNER JOIN users ON users.id = posts.user_id
		LEFT JOIN post_categories ON post_categories.post_id = posts.id
		LEFT JOIN categories ON categories.id = post_categories.category_id
		LEFT JOIN post_likes ON post_likes.post_id = posts.id
		LEFT JOIN comments ON comments.post_id = posts.id
		GROUP BY posts.id
		ORDER BY posts.created_at DESC
		LIMIT 50;
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load posts")
		return
	}
	defer rows.Close()

	// Start with an empty slice so JSON returns [] instead of null.
	posts := []postResponse{}

	for rows.Next() {
		var post postResponse
		var categoriesText string

		// Scan copies the current database row into Go variables.
		err := rows.Scan(
			&post.ID,
			&post.Title,
			&post.Content,
			&post.AuthorID,
			&post.Author,
			&categoriesText,
			&post.LikeCount,
			&post.CommentCount,
			&post.CreatedAt,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read post")
			return
		}

		// GROUP_CONCAT gives categories as one string, so split it back into []string.
		post.Categories = splitCategories(categoriesText)
		posts = append(posts, post)
	}

	// rows.Err catches errors that can happen during the loop.
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to finish reading posts")
		return
	}

	// writeJSON converts the Go response struct into JSON for the browser.
	writeJSON(w, http.StatusOK, postsFeedResponse{
		Posts: posts,
	})
}

func (req *createPostRequest) clean() {
	// Pointer receiver (*) lets this method update the original request struct.
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)

	cleanCategories := []string{}
	seen := map[string]bool{}

	for _, category := range req.Categories {
		// Remove spaces and ignore empty category names.
		category = strings.TrimSpace(category)

		if category == "" {
			continue
		}

		// Use lowercase for duplicate checks, so "Go" and "go" count as the same.
		key := strings.ToLower(category)
		if seen[key] {
			continue
		}

		seen[key] = true
		cleanCategories = append(cleanCategories, category)
	}

	req.Categories = cleanCategories
}

func (req createPostRequest) validate() error {
	// These errors are sent back to the frontend as JSON error messages.
	if req.Title == "" {
		return errors.New("title is required")
	}

	if req.Content == "" {
		return errors.New("content is required")
	}

	if len(req.Categories) == 0 {
		return errors.New("at least one category is required")
	}

	return nil
}

func getOrCreateCategoryID(tx *sql.Tx, name string) (int64, error) {
	// INSERT OR IGNORE creates the category only if it does not already exist.
	_, err := tx.Exec(`
		INSERT OR IGNORE INTO categories (name)
		VALUES (?);
	`, name)
	if err != nil {
		return 0, err
	}

	var categoryID int64

	// Whether it was new or already existed, read the category id.
	err = tx.QueryRow(`
		SELECT id
		FROM categories
		WHERE name = ?;
	`, name).Scan(&categoryID)
	if err != nil {
		return 0, err
	}

	return categoryID, nil
}

func splitCategories(categoriesText string) []string {
	// GROUP_CONCAT returns an empty string when there are no categories.
	if categoriesText == "" {
		return []string{}
	}

	// Convert "go,sqlite,frontend" into []string{"go", "sqlite", "frontend"}.
	parts := strings.Split(categoriesText, ",")
	categories := []string{}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			categories = append(categories, part)
		}
	}

	return categories
}

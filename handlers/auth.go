package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type registerRequest struct {
	// The json tags tell Go which JSON field should fill each struct field.
	// Example: {"first_name":"Ahmed"} becomes req.FirstName.
	Nickname  string `json:"nickname"`
	Age       int    `json:"age"`
	Gender    string `json:"gender"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

type registerResponse struct {
	// These tags control the names used when this struct is sent back as JSON.
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
	Message  string `json:"message"`
}

type loginRequest struct {
	// The frontend can send a nickname or an email in this one JSON field.
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type loginResponse struct {
	// This is the JSON shape the browser receives after a successful login.
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
	Message  string `json:"message"`
}

func (app *App) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// req starts as an empty Go struct. readJSON fills it from the request body.
	var req registerRequest

	// The & means "pass the address of req", so json.Decoder can modify it.
	err := readJSON(w, r, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Clean and validate after decoding, because JSON values may contain spaces
	// or missing/invalid fields.
	req.clean()

	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	result, err := app.DB.Exec(`
		INSERT INTO users (
			nickname,
			age,
			gender,
			first_name,
			last_name,
			email,
			password_hash
		)
		VALUES (?, ?, ?, ?, ?, ?, ?);
	`,
		req.Nickname,
		req.Age,
		req.Gender,
		req.FirstName,
		req.LastName,
		req.Email,
		string(passwordHash),
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			writeError(w, http.StatusConflict, "nickname or email already exists")
			return
		}

		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	userID, err := result.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read created user id")
		return
	}

	// writeJSON converts this Go struct into a JSON response for the browser.
	writeJSON(w, http.StatusCreated, registerResponse{
		ID:       userID,
		Nickname: req.Nickname,
		Message:  "user registered successfully",
	})
}

func (app *App) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// req will hold the JSON login details sent by the browser.
	var req loginRequest

	// Decode JSON like {"identifier":"ahmed","password":"secret123"} into req.
	err := readJSON(w, r, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.clean()

	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var userID int
	var nickname string
	var passwordHash string

	// Query the database for the user with the given nickname or email
	err = app.DB.QueryRow(`
		SELECT id, nickname, password_hash
		FROM users
		WHERE nickname = ? OR email = ?
		LIMIT 1;
	`, req.Identifier, req.Identifier).Scan(&userID, &nickname, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid nickname/email or password")
			return
		}

		writeError(w, http.StatusInternalServerError, "failed to find user")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid nickname/email or password")
		return
	}

	// Create a new session for the user
	sessionID := uuid.NewString()

	// Set the session to expire in 7 days
	expiresAt := time.Now().Add(7 * 24 * time.Hour).UTC()

	// Saving the session
	_, err = app.DB.Exec(`
		INSERT INTO sessions (id, user_id, expires_at)
		VALUES (?, ?, ?);
	`, sessionID, userID, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	// This sends the session ID to the browser
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Send a JSON success message after the cookie has been created.
	writeJSON(w, http.StatusOK, loginResponse{
		ID:       userID,
		Nickname: nickname,
		Message:  "logged in successfully",
	})
}

func (req *registerRequest) clean() {
	// Pointer receiver (*) lets this method update the original request struct.
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Gender = strings.TrimSpace(req.Gender)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
}

func (req registerRequest) validate() error {
	if req.Nickname == "" {
		return errors.New("nickname is required")
	}

	if req.Age <= 0 {
		return errors.New("age must be greater than 0")
	}

	if req.Gender == "" {
		return errors.New("gender is required")
	}

	if req.FirstName == "" {
		return errors.New("first name is required")
	}

	if req.LastName == "" {
		return errors.New("last name is required")
	}

	if req.Email == "" {
		return errors.New("email is required")
	}

	if !strings.Contains(req.Email, "@") {
		return errors.New("email is invalid")
	}

	if len(req.Password) < 6 {
		return errors.New("password must be at least 6 characters")
	}

	return nil
}

func (req *loginRequest) clean() {
	// Trim spaces so " ahmed " is treated the same as "ahmed".
	req.Identifier = strings.TrimSpace(req.Identifier)
	req.Password = strings.TrimSpace(req.Password)
}

func (req loginRequest) validate() error {
	if req.Identifier == "" {
		return errors.New("nickname or email is required")
	}

	if req.Password == "" {
		return errors.New("password is required")
	}

	return nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

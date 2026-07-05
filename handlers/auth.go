package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type registerRequest struct {
	Nickname  string `json:"nickname"`
	Age       int    `json:"age"`
	Gender    string `json:"gender"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

type registerResponse struct {
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
	Message  string `json:"message"`
}

func (app *App) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Then it reads JSON into: var req registerRequest
	// {"nickname": "ahmed",
	// "age": 22,
	// "gender": "male",
	// "first_name": "Ahmed",
	// "last_name": "Taha",
	// "email": "ahmed@example.com",
	// "password": "secret123"}
	var req registerRequest

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

	writeJSON(w, http.StatusCreated, registerResponse{
		ID:       userID,
		Nickname: req.Nickname,
		Message:  "user registered successfully",
	})
}

func (req *registerRequest) clean() {
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

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

var _ = sql.ErrNoRows

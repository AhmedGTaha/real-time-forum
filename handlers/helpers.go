package handlers

import (
	"encoding/json"
	"net/http"
)

type errorResponse struct {
	Error string `json:"error"`
}

// This reads the JSON request body and puts the data into a struct
func readJSON(w http.ResponseWriter, r *http.Request, data any) error {
	// Limit the size of the request body to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	decoder := json.NewDecoder(r.Body)

	// decoder.DisallowUnknownFields() rejects any JSON fields that do not match the struct
	decoder.DisallowUnknownFields()

	return decoder.Decode(data)
}

// This sends JSON responses back to the browser.
// Example response:
//
//	{
//	 "id": 1,
//	 "nickname": "ahmed",
//	 "message": "user registered successfully"
//	}
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{
		Error: message,
	})
}

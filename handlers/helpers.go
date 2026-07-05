package handlers

import (
	"encoding/json"
	"net/http"
)

type errorResponse struct {
	// writeError uses this struct so every error response has the same JSON shape:
	// {"error":"something went wrong"}
	Error string `json:"error"`
}

// readJSON reads the JSON request body and puts the data into the struct passed
// in as data. data should usually be a pointer, like &req, so Decode can fill it.
func readJSON(w http.ResponseWriter, r *http.Request, data any) error {
	// Limit the request body to 1MB so someone cannot send a huge JSON payload.
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	// A decoder reads JSON from the HTTP body one field at a time.
	decoder := json.NewDecoder(r.Body)

	// Reject JSON fields that do not match the struct tags.
	// Example: {"username":"ahmed"} fails if the struct expects "nickname".
	decoder.DisallowUnknownFields()

	// Decode is the moment where JSON becomes Go data.
	// For example, it fills registerRequest.Nickname from the "nickname" field.
	return decoder.Decode(data)
}

// writeJSON sends JSON responses back to the browser.
// Example response:
//
//	{
//	  "id": 1,
//	  "nickname": "ahmed",
//	  "message": "user registered successfully"
//	}
func writeJSON(w http.ResponseWriter, status int, data any) {
	// Tell the browser that the response body is JSON, not HTML or plain text.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Encode is the opposite of Decode: it turns Go data into JSON text.
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	// Reuse writeJSON so errors are sent in the same JSON format every time.
	writeJSON(w, status, errorResponse{
		Error: message,
	})
}

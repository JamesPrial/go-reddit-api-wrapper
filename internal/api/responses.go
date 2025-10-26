package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// SuccessResponse wraps successful API responses with a data field.
// This provides a consistent response structure across all endpoints.
type SuccessResponse struct {
	Data interface{} `json:"data"`
}

// ErrorResponse wraps error responses with error details.
// The Error field contains a machine-readable error code,
// while Message provides a human-readable description.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// WriteJSON writes a JSON response with the specified status code.
// It sets appropriate headers and handles encoding errors.
// If encoding fails, it writes a 500 Internal Server Error response.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response",
			"error", err,
			"status", status)
	}
}

// WriteSuccess writes a successful JSON response with the provided data.
// The data is wrapped in a SuccessResponse structure.
func WriteSuccess(w http.ResponseWriter, status int, data interface{}) {
	WriteJSON(w, status, SuccessResponse{Data: data})
}

// WriteError writes an error JSON response with the specified status code.
// It creates an ErrorResponse with the provided error code and message.
func WriteError(w http.ResponseWriter, status int, errorCode, message string) {
	WriteJSON(w, status, ErrorResponse{
		Error:   errorCode,
		Message: message,
	})
}

// WriteBadRequest writes a 400 Bad Request error response.
// This is used for invalid request parameters or malformed input.
func WriteBadRequest(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusBadRequest, "bad_request", message)
}

// WriteNotFound writes a 404 Not Found error response.
// This is used when a requested resource does not exist.
func WriteNotFound(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusNotFound, "not_found", message)
}

// WriteInternalError writes a 500 Internal Server Error response.
// This is used for unexpected errors that occur during request processing.
func WriteInternalError(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusInternalServerError, "internal_error", message)
}

// WriteConflict writes a 409 Conflict error response.
// This is used when a request conflicts with the current state of the server.
func WriteConflict(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusConflict, "conflict", message)
}

// WriteNoContent writes a 204 No Content response with no body.
// This is used for successful operations that don't return data.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

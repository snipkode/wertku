package response

import (
	"encoding/json"
	"net/http"

	"github.com/snipkode/wertku/internal/apperror"
)

// APIResponse is the standard JSON envelope for all API responses.
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// JSON writes a successful JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{Success: true, Data: data}) //nolint:errcheck
}

// Error maps an application error to an HTTP status code and writes the response.
// Raw database or internal errors must never reach this function —
// they should be wrapped in application errors upstream.
func Error(w http.ResponseWriter, err error) {
	status := apperror.HTTPStatus(err)
	ErrorMsg(w, status, err.Error())
}

// ErrorMsg writes an error response with an explicit status code and message.
func ErrorMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{Success: false, Error: msg}) //nolint:errcheck
}

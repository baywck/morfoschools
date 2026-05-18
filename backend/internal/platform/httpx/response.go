package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody holds error details.
type ErrorBody struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"requestId,omitempty"`
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteError writes a standard error response.
func WriteError(w http.ResponseWriter, status int, code, message, requestID string) {
	WriteJSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	})
}

// WriteValidationError writes a structured validation error with field-level messages.
func WriteValidationError(w http.ResponseWriter, fields map[string]string, requestID string) {
	WriteJSON(w, http.StatusUnprocessableEntity, ErrorResponse{
		Error: ErrorBody{
			Code:      "validation_failed",
			Message:   "Validation failed",
			Fields:    fields,
			RequestID: requestID,
		},
	})
}

// ReadJSON decodes a JSON request body.
func ReadJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("empty body")
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

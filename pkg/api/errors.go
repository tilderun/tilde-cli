package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// APIError represents a structured error from the Tilde API.
type APIError struct {
	StatusCode int    `json:"-"`
	Message    string `json:"message"`
	Code       string `json:"code"`
	RequestID  string `json:"request_id"`
}

func (e *APIError) Error() string {
	s := fmt.Sprintf("API error (HTTP %d)", e.StatusCode)
	if e.Code != "" {
		s += fmt.Sprintf(" [%s]", e.Code)
	}
	if e.Message != "" {
		s += ": " + e.Message
	}
	if e.RequestID != "" {
		s += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}
	return s
}

// parseAPIError attempts to parse an API error from an HTTP response.
// If parsing fails, it returns a generic error with the status code and body snippet.
func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	apiErr := &APIError{StatusCode: resp.StatusCode}
	if err := json.Unmarshal(body, apiErr); err != nil {
		apiErr.Message = string(body)
	}
	return apiErr
}

// IsNotFound returns true if the error is a 404 API error.
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

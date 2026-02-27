package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  APIError
		want string
	}{
		{
			name: "full error",
			err: APIError{
				StatusCode: 404,
				Message:    "object not found",
				Code:       "not_found",
				RequestID:  "req-123",
			},
			want: "API error (HTTP 404) [not_found]: object not found (request_id: req-123)",
		},
		{
			name: "message only",
			err: APIError{
				StatusCode: 400,
				Message:    "bad request",
			},
			want: "API error (HTTP 400): bad request",
		},
		{
			name: "code and message",
			err: APIError{
				StatusCode: 403,
				Message:    "forbidden",
				Code:       "access_denied",
			},
			want: "API error (HTTP 403) [access_denied]: forbidden",
		},
		{
			name: "status only",
			err: APIError{
				StatusCode: 500,
			},
			want: "API error (HTTP 500)",
		},
		{
			name: "request_id only",
			err: APIError{
				StatusCode: 500,
				RequestID:  "req-abc",
			},
			want: "API error (HTTP 500) (request_id: req-abc)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseAPIError_ValidJSON(t *testing.T) {
	body := `{"message":"not found","code":"not_found","request_id":"req-456"}`
	resp := &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	err := parseAPIError(resp)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}

	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "not found" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "not found")
	}
	if apiErr.Code != "not_found" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "not_found")
	}
	if apiErr.RequestID != "req-456" {
		t.Errorf("RequestID = %q, want %q", apiErr.RequestID, "req-456")
	}
}

func TestParseAPIError_InvalidJSON(t *testing.T) {
	body := "Internal Server Error"
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	err := parseAPIError(resp)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}

	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", apiErr.StatusCode)
	}
	if apiErr.Message != "Internal Server Error" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "Internal Server Error")
	}
}

func TestParseAPIError_EmptyBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: 502,
		Body:       io.NopCloser(strings.NewReader("")),
	}

	err := parseAPIError(resp)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 502 {
		t.Errorf("StatusCode = %d, want 502", apiErr.StatusCode)
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "404 API error",
			err:  &APIError{StatusCode: 404, Message: "not found"},
			want: true,
		},
		{
			name: "400 API error",
			err:  &APIError{StatusCode: 400, Message: "bad request"},
			want: false,
		},
		{
			name: "non-API error",
			err:  fmt.Errorf("some other error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNotFound(tt.err)
			if got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

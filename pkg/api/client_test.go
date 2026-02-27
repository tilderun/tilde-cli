package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("https://example.com/api/v1", "cak-testkey")

	if c.BaseURL != "https://example.com/api/v1" {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, "https://example.com/api/v1")
	}
	if c.APIKey != "cak-testkey" {
		t.Errorf("APIKey = %q, want %q", c.APIKey, "cak-testkey")
	}
	if c.HTTPClient == nil {
		t.Error("HTTPClient is nil")
	}
	if c.S3Client == nil {
		t.Error("S3Client is nil")
	}
}

func TestClient_do_SetsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-mykey")
	resp, err := c.do(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer cak-mykey" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer cak-mykey")
	}
}

func TestClient_do_SetsContentTypeForBody(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")

	// With body
	resp, err := c.do(context.Background(), http.MethodPost, "/test", bytes.NewReader([]byte(`{"a":"b"}`)))
	if err != nil {
		t.Fatalf("do with body: %v", err)
	}
	resp.Body.Close()
	if gotCT != "application/json" {
		t.Errorf("Content-Type with body = %q, want %q", gotCT, "application/json")
	}

	// Without body
	gotCT = ""
	resp, err = c.do(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("do without body: %v", err)
	}
	resp.Body.Close()
	if gotCT != "" {
		t.Errorf("Content-Type without body = %q, want empty", gotCT)
	}
}

func TestClient_do_DoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "https://s3.example.com/presigned")
			w.WriteHeader(307)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	resp, err := c.do(context.Background(), http.MethodGet, "/redirect", nil)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 307 {
		t.Errorf("StatusCode = %d, want 307 (should NOT follow redirect)", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "https://s3.example.com/presigned" {
		t.Errorf("Location = %q, want presigned URL", loc)
	}
}

func TestClient_doJSON_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"session_id": "abc-123"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")

	var resp CreateSessionResponse
	_, err := c.doJSON(context.Background(), http.MethodPost, "/sessions", nil, &resp)
	if err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if resp.SessionID != "abc-123" {
		t.Errorf("SessionID = %q, want %q", resp.SessionID, "abc-123")
	}
}

func TestClient_doJSON_WithRequestBody(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"commit_id": "c-1"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	reqBody := &CommitRequest{Message: "test commit"}

	var resp CommitResponse
	_, err := c.doJSON(context.Background(), http.MethodPost, "/commit", reqBody, &resp)
	if err != nil {
		t.Fatalf("doJSON: %v", err)
	}

	if gotBody["message"] != "test commit" {
		t.Errorf("request body message = %q, want %q", gotBody["message"], "test commit")
	}
	if resp.CommitID != "c-1" {
		t.Errorf("CommitID = %q, want %q", resp.CommitID, "c-1")
	}
}

func TestClient_doJSON_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{
			"message":    "forbidden",
			"code":       "access_denied",
			"request_id": "req-999",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	var resp CreateSessionResponse
	_, err := c.doJSON(context.Background(), http.MethodPost, "/sessions", nil, &resp)
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
	if apiErr.Code != "access_denied" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "access_denied")
	}
}

func TestClient_doJSON_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.doJSON(context.Background(), http.MethodDelete, "/something", nil, nil)
	if err != nil {
		t.Fatalf("doJSON with 204: %v", err)
	}
}

func TestClient_doJSON_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.doJSON(ctx, http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestClient_doRaw_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("file content"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	resp, err := c.doRaw(context.Background(), http.MethodGet, "/object", nil, "")
	if err != nil {
		t.Fatalf("doRaw: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "file content" {
		t.Errorf("body = %q, want %q", string(body), "file content")
	}
}

func TestClient_doRaw_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.doRaw(context.Background(), http.MethodGet, "/object", nil, "")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
}

func TestClient_doRaw_SetsContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	resp, err := c.doRaw(context.Background(), http.MethodPut, "/upload", nil, "application/octet-stream")
	if err != nil {
		t.Fatalf("doRaw: %v", err)
	}
	resp.Body.Close()

	if gotCT != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want %q", gotCT, "application/octet-stream")
	}
}

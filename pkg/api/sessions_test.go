package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/organizations/myorg/repositories/myrepo/sessions" {
			t.Errorf("path = %s, want /organizations/myorg/repositories/myrepo/sessions", r.URL.Path)
		}

		w.WriteHeader(201)
		json.NewEncoder(w).Encode(CreateSessionResponse{SessionID: "sess-abc"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	resp, err := c.CreateSession(context.Background(), "myorg", "myrepo")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if resp.SessionID != "sess-abc" {
		t.Errorf("SessionID = %q, want %q", resp.SessionID, "sess-abc")
	}
}

func TestCreateSession_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "forbidden",
			"code":    "access_denied",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.CreateSession(context.Background(), "myorg", "myrepo")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
}

func TestCommitSession_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		wantPath := "/organizations/org1/repositories/repo1/sessions/sess-1"
		if r.URL.Path != wantPath {
			t.Errorf("path = %s, want %s", r.URL.Path, wantPath)
		}

		var body CommitRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Message != "test commit" {
			t.Errorf("message = %q, want %q", body.Message, "test commit")
		}

		w.WriteHeader(200)
		json.NewEncoder(w).Encode(CommitResponse{CommitID: "commit-xyz"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	resp, err := c.CommitSession(context.Background(), "org1", "repo1", "sess-1", &CommitRequest{
		Message: "test commit",
	})
	if err != nil {
		t.Fatalf("CommitSession: %v", err)
	}
	if resp.CommitID != "commit-xyz" {
		t.Errorf("CommitID = %q, want %q", resp.CommitID, "commit-xyz")
	}
	if resp.ApprovalRequired {
		t.Error("ApprovalRequired should be false")
	}
}

func TestCommitSession_ApprovalRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"approval_required": true,
			"session_id":        "sess-1",
			"message":           "Approval required",
			"web_url":           "https://cerebral.example.com/approve/sess-1",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	resp, err := c.CommitSession(context.Background(), "org1", "repo1", "sess-1", &CommitRequest{
		Message: "needs approval",
	})
	if err != nil {
		t.Fatalf("CommitSession: %v", err)
	}
	if !resp.ApprovalRequired {
		t.Error("expected ApprovalRequired to be true")
	}
	if resp.WebURL != "https://cerebral.example.com/approve/sess-1" {
		t.Errorf("WebURL = %q", resp.WebURL)
	}
}

func TestCommitSession_WithMetadata(t *testing.T) {
	var gotBody CommitRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(CommitResponse{CommitID: "c-2"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.CommitSession(context.Background(), "org", "repo", "sess", &CommitRequest{
		Message:  "with meta",
		Metadata: map[string]string{"source": "ci", "pipeline": "main"},
	})
	if err != nil {
		t.Fatalf("CommitSession: %v", err)
	}

	if gotBody.Metadata["source"] != "ci" {
		t.Errorf("metadata source = %q, want %q", gotBody.Metadata["source"], "ci")
	}
	if gotBody.Metadata["pipeline"] != "main" {
		t.Errorf("metadata pipeline = %q, want %q", gotBody.Metadata["pipeline"], "main")
	}
}

func TestCommitSession_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "conflict",
			"code":    "conflict",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.CommitSession(context.Background(), "org", "repo", "sess", &CommitRequest{Message: "msg"})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 409 {
		t.Errorf("StatusCode = %d, want 409", apiErr.StatusCode)
	}
}

func TestRollbackSession(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	err := c.RollbackSession(context.Background(), "org1", "repo1", "sess-99")
	if err != nil {
		t.Fatalf("RollbackSession: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	wantPath := "/organizations/org1/repositories/repo1/sessions/sess-99"
	if gotPath != wantPath {
		t.Errorf("path = %s, want %s", gotPath, wantPath)
	}
}

func TestRollbackSession_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "session not found",
			"code":    "not_found",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	err := c.RollbackSession(context.Background(), "org", "repo", "bad-sess")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected not found error, got: %v", err)
	}
}

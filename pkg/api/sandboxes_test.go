package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateSandbox(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/organizations/myorg/repositories/myrepo/sandboxes") {
			t.Errorf("path = %s", r.URL.Path)
		}
		var req CreateSandboxRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Image != "alpine" {
			t.Errorf("image = %q, want alpine", req.Image)
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(CreateSandboxResponse{SandboxID: "sb-123"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	resp, err := c.CreateSandbox(context.Background(), "myorg", "myrepo", CreateSandboxRequest{
		Image:   "alpine",
		Command: []string{"echo", "hello"},
	})
	if err != nil {
		t.Fatalf("CreateSandbox: %v", err)
	}
	if resp.SandboxID != "sb-123" {
		t.Errorf("SandboxID = %q, want sb-123", resp.SandboxID)
	}
}

func TestGetSandbox(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		json.NewEncoder(w).Encode(Sandbox{
			ID:     "sb-123",
			Image:  "alpine",
			Status: "running",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	sb, err := c.GetSandbox(context.Background(), "org", "repo", "sb-123")
	if err != nil {
		t.Fatalf("GetSandbox: %v", err)
	}
	if sb.ID != "sb-123" {
		t.Errorf("ID = %q, want sb-123", sb.ID)
	}
	if sb.Status != "running" {
		t.Errorf("Status = %q, want running", sb.Status)
	}
}

func TestCancelSandbox(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(202)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	err := c.CancelSandbox(context.Background(), "org", "repo", "sb-123")
	if err != nil {
		t.Fatalf("CancelSandbox: %v", err)
	}
}

func TestCancelSandbox_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	err := c.CancelSandbox(context.Background(), "org", "repo", "sb-999")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSandboxStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/status") {
			t.Errorf("path = %s, want /status suffix", r.URL.Path)
		}
		exitCode := 0
		json.NewEncoder(w).Encode(SandboxStatusResponse{
			Status:   "completed",
			ExitCode: &exitCode,
			CommitID: "c-abc",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	status, err := c.GetSandboxStatus(context.Background(), "org", "repo", "sb-123")
	if err != nil {
		t.Fatalf("GetSandboxStatus: %v", err)
	}
	if status.Status != "completed" {
		t.Errorf("Status = %q, want completed", status.Status)
	}
	if status.ExitCode == nil || *status.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", status.ExitCode)
	}
}

func TestStreamSandboxOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/combined") {
			t.Errorf("path = %s, want /combined suffix", r.URL.Path)
		}
		w.Write([]byte("hello from sandbox\n"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	rc, err := c.StreamSandboxOutput(context.Background(), "org", "repo", "sb-123", "combined")
	if err != nil {
		t.Fatalf("StreamSandboxOutput: %v", err)
	}
	defer rc.Close()

	data, _ := io.ReadAll(rc)
	if string(data) != "hello from sandbox\n" {
		t.Errorf("output = %q, want %q", string(data), "hello from sandbox\n")
	}
}

func TestTerminalWebSocketURL(t *testing.T) {
	c := NewClient("https://tilde.run/api/v1", "tuk-key")
	got := c.TerminalWebSocketURL("myorg", "myrepo", "sb-123")
	want := "wss://tilde.run/api/v1/organizations/myorg/repositories/myrepo/sandboxes/sb-123/terminal"
	if got != want {
		t.Errorf("TerminalWebSocketURL = %q, want %q", got, want)
	}
}

func TestTerminalWebSocketURL_HTTP(t *testing.T) {
	c := NewClient("http://localhost:8080/api/v1", "tuk-key")
	got := c.TerminalWebSocketURL("org", "repo", "sb-1")
	want := "ws://localhost:8080/api/v1/organizations/org/repositories/repo/sandboxes/sb-1/terminal"
	if got != want {
		t.Errorf("TerminalWebSocketURL = %q, want %q", got, want)
	}
}

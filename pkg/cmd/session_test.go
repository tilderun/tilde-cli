package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
)

// setupTestEnv creates a mock API server and configures the env to point at it.
// Returns a cleanup function.
func setupTestEnv(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("CEREBRAL_API_KEY", "cak-testkey")
	t.Setenv("CEREBRAL_ENDPOINT_URL", srv.URL)
}

func TestSessionStart(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/organizations/myorg/repositories/myrepo/sessions") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(api.CreateSessionResponse{SessionID: "sess-new"})
	})

	var buf bytes.Buffer
	root := NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"session", "start", "cb://myorg/myrepo"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSessionStart_InvalidURI(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "start", "not-a-uri"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid URI")
	}
}

func TestSessionStart_MissingArg(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "start"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestSessionCommit_Success(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(api.CommitResponse{CommitID: "commit-abc"})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"session", "commit", "--session", "sess-1", "-m", "test msg", "cb://org/repo"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSessionCommit_ApprovalRequired(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"approval_required": true,
			"web_url":           "https://example.com/approve",
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"session", "commit", "--session", "sess-1", "-m", "msg", "cb://org/repo"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSessionCommit_MissingSession(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "commit", "-m", "msg", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --session")
	}
}

func TestSessionCommit_MissingMessage(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "commit", "--session", "sess-1", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing -m")
	}
}

func TestSessionRollback_Success(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(204)
	})

	root := NewRootCmd()
	root.SetArgs([]string{"session", "rollback", "--session", "sess-1", "cb://org/repo"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSessionRollback_MissingSession(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "rollback", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --session")
	}
}

func TestSessionRollback_APIError(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"message": "session not found"})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"session", "rollback", "--session", "bad-sess", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestSessionSubcommands(t *testing.T) {
	root := NewRootCmd()
	sessionCmd, _, err := root.Find([]string{"session"})
	if err != nil {
		t.Fatalf("Find session: %v", err)
	}

	subcommands := make(map[string]bool)
	for _, cmd := range sessionCmd.Commands() {
		subcommands[cmd.Name()] = true
	}

	for _, name := range []string{"start", "commit", "rollback"} {
		if !subcommands[name] {
			t.Errorf("missing subcommand: session %s", name)
		}
	}
}

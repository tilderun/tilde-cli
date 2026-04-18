package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/tilderun/tilde-cli/pkg/api"
)

func TestSandboxLogs(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			_ = json.NewEncoder(w).Encode(api.SandboxStatusResponse{Status: api.SandboxStatusRunning})
		case strings.HasSuffix(r.URL.Path, "/combined"):
			w.Write([]byte("log line 1\nlog line 2\n"))
		default:
			t.Errorf("unexpected path = %s", r.URL.Path)
		}
	})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "logs", "-r", "org/repo", "sb-123"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

// TestSandboxLogs_WaitsForStart verifies the CLI polls status and only
// fetches logs once the sandbox has left the `starting` state.
func TestSandboxLogs_WaitsForStart(t *testing.T) {
	var statusCalls int
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			statusCalls++
			s := api.SandboxStatusStarting
			if statusCalls > 1 {
				s = api.SandboxStatusRunning
			}
			_ = json.NewEncoder(w).Encode(api.SandboxStatusResponse{Status: s})
		case strings.HasSuffix(r.URL.Path, "/combined"):
			w.Write([]byte("ok\n"))
		default:
			t.Errorf("unexpected path = %s", r.URL.Path)
		}
	})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "logs", "-r", "org/repo", "sb-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if statusCalls < 2 {
		t.Errorf("status calls = %d, want >= 2 (expected to wait past starting)", statusCalls)
	}
}

// TestSandboxLogs_TerminalStateReadsSnapshot verifies that logs can still be
// fetched after the sandbox has reached a terminal state.
func TestSandboxLogs_TerminalStateReadsSnapshot(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			_ = json.NewEncoder(w).Encode(api.SandboxStatusResponse{Status: api.SandboxStatusCommitted})
		case strings.HasSuffix(r.URL.Path, "/combined"):
			w.Write([]byte("final output\n"))
		default:
			t.Errorf("unexpected path = %s", r.URL.Path)
		}
	})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "logs", "-r", "org/repo", "sb-123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSandboxLogs_MissingRepo(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "logs", "sb-123"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --repo")
	}
}

func TestSandboxLogs_MissingSandboxID(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "logs", "-r", "org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing sandbox ID")
	}
}

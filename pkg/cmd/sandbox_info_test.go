package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/tilderun/tilde-cli/pkg/api"
)

func TestSandboxInfo(t *testing.T) {
	exitCode := 0
	finished := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.Sandbox{
			ID:        "sb-123",
			Image:     "alpine",
			Status:    "completed",
			Command:   []string{"echo", "hello"},
			ExitCode:  &exitCode,
			CommitID:  "c-abc",
			CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			FinishedAt: &finished,
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "info", "-r", "org/repo", "sb-123"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSandboxInfo_MissingRepo(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "info", "sb-123"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --repo")
	}
}

func TestSandboxInfo_MissingSandboxID(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "info", "-r", "org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing sandbox ID")
	}
}

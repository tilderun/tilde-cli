package cmd

import (
	"net/http"
	"strings"
	"testing"
)

func TestSandboxLogs(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/combined") {
			t.Errorf("path = %s, want /combined suffix", r.URL.Path)
		}
		w.Write([]byte("log line 1\nlog line 2\n"))
	})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "logs", "-r", "org/repo", "sb-123"})
	err := root.Execute()
	if err != nil {
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

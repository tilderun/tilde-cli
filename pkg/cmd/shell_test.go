package cmd

import (
	"net/http"
	"testing"
)

func TestShellCmd_InvalidRepo(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"shell", "noslash"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid repo format")
	}
}

func TestShellCmd_MissingArg(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"shell"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument")
	}
}

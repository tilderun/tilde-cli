package cmd

import (
	"net/http"
	"testing"
)

func TestExecCmd_InvalidRepo(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"exec", "noslash", "--", "echo", "hello"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid repo format")
	}
}

func TestExecCmd_MissingArgs(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"exec"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing arguments")
	}
}

package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/tilderun/tilde-cli/pkg/api"
)

func TestSandboxRun_Detach(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var req api.CreateSandboxRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Image != "alpine" {
			t.Errorf("image = %q, want alpine", req.Image)
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(api.CreateSandboxResponse{SandboxID: "sb-abc"})
	})

	var buf bytes.Buffer
	root := NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"sandbox", "run", "-r", "myorg/myrepo", "--image", "alpine", "-d", "--", "echo", "hello"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSandboxRun_MissingRepo(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "run", "--image", "alpine"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --repo")
	}
}

func TestSandboxRun_MissingImage(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "run", "-r", "org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --image")
	}
}

func TestSandboxRun_InvalidEnvVar(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "run", "-r", "org/repo", "--image", "alpine", "-e", "NOEQUALS"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid env var")
	}
}

func TestSandboxRun_EnvVars(t *testing.T) {
	var gotReq api.CreateSandboxRequest
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(api.CreateSandboxResponse{SandboxID: "sb-env"})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"sandbox", "run", "-r", "org/repo", "--image", "alpine", "-d",
		"-e", "FOO=bar", "-e", "BAZ=qux"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotReq.EnvVars["FOO"] != "bar" {
		t.Errorf("FOO = %q, want bar", gotReq.EnvVars["FOO"])
	}
	if gotReq.EnvVars["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want qux", gotReq.EnvVars["BAZ"])
	}
}

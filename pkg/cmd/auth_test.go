package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/tilderun/tilde-cli/pkg/config"
)

func TestAuthStatus_NotLoggedIn(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TILDE_API_KEY", "")
	t.Setenv("TILDE_ENDPOINT_URL", "")

	root := NewRootCmd()
	root.SetArgs([]string{"auth", "status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auth status: %v", err)
	}
}

func TestAuthStatus_LoggedIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/me" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]any{
					"username": "testuser",
					"email":    "test@example.com",
				},
				"organizations": []any{},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TILDE_API_KEY", "")
	t.Setenv("TILDE_ENDPOINT_URL", srv.URL)

	// Write config with token
	if err := config.Save(&config.Config{APIKey: "tuk-testtoken"}); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"auth", "status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auth status: %v", err)
	}
}

func TestAuthLogout(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TILDE_API_KEY", "")
	t.Setenv("TILDE_ENDPOINT_URL", "")

	// Create config first
	if err := config.Save(&config.Config{APIKey: "tuk-old"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"auth", "logout"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auth logout: %v", err)
	}

	// Verify config is gone
	path := filepath.Join(tmp, ".tilde", "config.yaml")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("config file should be removed after logout")
	}
}

func TestAuthLogin_DeviceFlow(t *testing.T) {
	var pollCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/device/code":
			json.NewEncoder(w).Encode(deviceCodeResponse{
				DeviceCode:              "dev-123",
				UserCode:                "ABCD-1234",
				VerificationURIComplete: "https://example.com/verify?code=ABCD-1234",
				ExpiresIn:               300,
				Interval:                1,
			})
		case "/api/v1/auth/device/token":
			count := pollCount.Add(1)
			if count < 2 {
				json.NewEncoder(w).Encode(deviceTokenResponse{Error: "authorization_pending"})
			} else {
				json.NewEncoder(w).Encode(deviceTokenResponse{AccessToken: "tuk-newtoken123"})
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TILDE_API_KEY", "")
	t.Setenv("TILDE_ENDPOINT_URL", srv.URL)

	// Redirect stdin so Scanln doesn't block
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.Write([]byte("\n"))
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	root := NewRootCmd()
	root.SetArgs([]string{"auth", "login"})
	if err := root.Execute(); err != nil {
		t.Fatalf("auth login: %v", err)
	}

	// Verify config was written
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	if cfg.APIKey != "tuk-newtoken123" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "tuk-newtoken123")
	}
}

func TestRootCmd_APIKeyFromConfig(t *testing.T) {
	// Verify that when TILDE_API_KEY is not set, the config file is used
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer tuk-fromconfig" {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results":    []any{},
			"pagination": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TILDE_API_KEY", "")
	t.Setenv("TILDE_ENDPOINT_URL", srv.URL)

	if err := config.Save(&config.Config{APIKey: "tuk-fromconfig"}); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls"})
	if err := root.Execute(); err != nil {
		t.Fatalf("repository ls with config key: %v", err)
	}
}

func TestRootCmd_APIKeyFlagPrecedence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer tuk-fromflag" {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results":    []any{},
			"pagination": map[string]any{"has_more": false},
		})
	}))
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TILDE_API_KEY", "tuk-fromenv")
	t.Setenv("TILDE_ENDPOINT_URL", srv.URL)

	if err := config.Save(&config.Config{APIKey: "tuk-fromconfig"}); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"--api-key", "tuk-fromflag", "repository", "ls"})
	if err := root.Execute(); err != nil {
		t.Fatalf("repository ls with --api-key: %v", err)
	}
}

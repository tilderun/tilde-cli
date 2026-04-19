package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tilderun/tilde-cli/pkg/api"
)

func TestRootCmd_MissingAPIKey(t *testing.T) {
	t.Setenv("TILDE_API_KEY", "")
	t.Setenv("HOME", t.TempDir())

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestRootCmd_InvalidAPIKeyPrefix(t *testing.T) {
	t.Setenv("TILDE_API_KEY", "bad-prefix-key")
	t.Setenv("HOME", t.TempDir())

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid API key prefix")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestRootCmd_ValidAPIKey(t *testing.T) {
	t.Setenv("TILDE_API_KEY", "tuk-test1234")
	root := NewRootCmd()
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}
}

func TestRootCmd_ValidAPIKey_AllPrefixes(t *testing.T) {
	for _, prefix := range []string{"tuk-", "trk-", "tak-"} {
		t.Run(prefix, func(t *testing.T) {
			t.Setenv("TILDE_API_KEY", prefix+"test1234")
			root := NewRootCmd()
			root.SetArgs([]string{"--help"})
			err := root.Execute()
			if err != nil {
				t.Fatalf("help should not error: %v", err)
			}
		})
	}
}

func TestRootCmd_CustomEndpoint(t *testing.T) {
	t.Setenv("TILDE_API_KEY", "tuk-test1234")
	t.Setenv("TILDE_ENDPOINT_URL", "https://custom.example.com/")

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls"})
	// It will error on the actual HTTP call but we just want PreRunE to pass
	_ = root.Execute()

	if apiClient == nil {
		t.Fatal("apiClient should be initialized")
	}
	if apiClient.BaseURL != "https://custom.example.com/api/v1" {
		t.Errorf("BaseURL = %q, want %q", apiClient.BaseURL, "https://custom.example.com/api/v1")
	}
}

func TestRootCmd_HasAllSubcommands(t *testing.T) {
	root := NewRootCmd()

	subcommands := make(map[string]bool)
	for _, cmd := range root.Commands() {
		subcommands[cmd.Name()] = true
	}

	expected := []string{"sandbox", "repository", "shell", "exec", "auth"}
	for _, name := range expected {
		if !subcommands[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestRootCmd_SandboxCredentialsURI(t *testing.T) {
	// When running inside a sandbox, TILDE_SANDBOX_CREDENTIALS_URI short-
	// circuits the API key flow. The CLI fetches creds from the metadata
	// endpoint and uses the returned api_url as the API base.
	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.Credentials{
			AccessToken:    "tst-from-metadata",
			ExpiresAt:      time.Now().Add(15 * time.Minute),
			PrincipalType:  "agent",
			PrincipalID:    "p-1",
			PrincipalName:  "deploy-bot",
			OrganizationID: "org-1",
			APIURL:         "https://sandbox-api.example.com",
		})
	}))
	defer metadata.Close()

	t.Setenv("TILDE_API_KEY", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TILDE_SANDBOX_CREDENTIALS_URI", metadata.URL)

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls"})
	// Will fail when hitting the fake api URL, but we only care about
	// PersistentPreRunE successfully setting up the client.
	_ = root.Execute()

	if apiClient == nil {
		t.Fatal("apiClient should be initialized from metadata response")
	}
	if apiClient.BaseURL != "https://sandbox-api.example.com/api/v1" {
		t.Errorf("BaseURL = %q, want %q", apiClient.BaseURL, "https://sandbox-api.example.com/api/v1")
	}
	tok, err := apiClient.Token(t.Context())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "tst-from-metadata" {
		t.Errorf("Token = %q, want %q", tok, "tst-from-metadata")
	}
}

func TestRootCmd_SandboxCredentialsURI_FlagOverrides(t *testing.T) {
	// An explicit --api-key should win over the metadata URI so that
	// humans debugging inside a sandbox can still target a specific key.
	t.Setenv("TILDE_API_KEY", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TILDE_SANDBOX_CREDENTIALS_URI", "http://127.0.0.1:1/should-not-be-called")

	root := NewRootCmd()
	root.SetArgs([]string{"--api-key", "tuk-explicit", "repository", "ls"})
	_ = root.Execute()

	if apiClient == nil {
		t.Fatal("apiClient should be initialized")
	}
	tok, err := apiClient.Token(t.Context())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "tuk-explicit" {
		t.Errorf("Token = %q, want %q (--api-key flag should win)", tok, "tuk-explicit")
	}
}

func TestRootCmd_HelpDoesNotRequireAPIKey(t *testing.T) {
	t.Setenv("TILDE_API_KEY", "")
	t.Setenv("HOME", t.TempDir())

	root := NewRootCmd()
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("help should not require API key: %v", err)
	}
}

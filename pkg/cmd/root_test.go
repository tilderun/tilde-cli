package cmd

import (
	"os"
	"testing"
)

func TestRootCmd_MissingAPIKey(t *testing.T) {
	os.Unsetenv("TILDE_API_KEY")

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

	expected := []string{"sandbox", "repository", "shell", "exec"}
	for _, name := range expected {
		if !subcommands[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestRootCmd_HelpDoesNotRequireAPIKey(t *testing.T) {
	os.Unsetenv("TILDE_API_KEY")

	root := NewRootCmd()
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("help should not require API key: %v", err)
	}
}

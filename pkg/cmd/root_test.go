package cmd

import (
	"os"
	"testing"
)

func TestRootCmd_MissingAPIKey(t *testing.T) {
	os.Unsetenv("CEREBRAL_API_KEY")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "start", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestRootCmd_InvalidAPIKeyPrefix(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "bad-prefix-key")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "start", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid API key prefix")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestRootCmd_ValidAPIKey(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test1234")
	// This will fail at the network level, but should pass the PersistentPreRunE
	root := NewRootCmd()
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}
}

func TestRootCmd_CustomEndpoint(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test1234")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "https://custom.example.com/")

	root := NewRootCmd()
	// Use session start to trigger PersistentPreRunE which sets up the client
	root.SetArgs([]string{"session", "start", "cb://org/repo"})
	// It will error on the actual HTTP call but we just want PreRunE to pass
	_ = root.Execute()

	if apiClient == nil {
		t.Fatal("apiClient should be initialized")
	}
	if apiClient.BaseURL != "https://custom.example.com/api/v1" {
		t.Errorf("BaseURL = %q, want %q", apiClient.BaseURL, "https://custom.example.com/api/v1")
	}
}

func TestRootCmd_CustomConcurrency(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test1234")
	t.Setenv("CEREBRAL_CLI_MAX_CONCURRENCY", "8")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "start", "cb://org/repo"})
	_ = root.Execute()

	if maxConcurrency != 8 {
		t.Errorf("maxConcurrency = %d, want 8", maxConcurrency)
	}
}

func TestRootCmd_InvalidConcurrency(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test1234")
	t.Setenv("CEREBRAL_CLI_MAX_CONCURRENCY", "notanumber")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "start", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid concurrency")
	}
}

func TestRootCmd_ZeroConcurrency(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test1234")
	t.Setenv("CEREBRAL_CLI_MAX_CONCURRENCY", "0")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "start", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for zero concurrency")
	}
}

func TestRootCmd_NegativeConcurrency(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test1234")
	t.Setenv("CEREBRAL_CLI_MAX_CONCURRENCY", "-1")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "start", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for negative concurrency")
	}
}

func TestRootCmd_DefaultConcurrency(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test1234")
	os.Unsetenv("CEREBRAL_CLI_MAX_CONCURRENCY")

	root := NewRootCmd()
	root.SetArgs([]string{"session", "start", "cb://org/repo"})
	_ = root.Execute()

	if maxConcurrency != defaultConcurrency {
		t.Errorf("maxConcurrency = %d, want %d", maxConcurrency, defaultConcurrency)
	}
}

func TestRootCmd_HasAllSubcommands(t *testing.T) {
	root := NewRootCmd()

	subcommands := make(map[string]bool)
	for _, cmd := range root.Commands() {
		subcommands[cmd.Name()] = true
	}

	expected := []string{"session", "cp", "rm", "ls"}
	for _, name := range expected {
		if !subcommands[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestRootCmd_HelpDoesNotRequireAPIKey(t *testing.T) {
	os.Unsetenv("CEREBRAL_API_KEY")

	root := NewRootCmd()
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("help should not require API key: %v", err)
	}
}

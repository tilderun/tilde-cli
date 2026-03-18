package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	// Point home to a temp dir with no config
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", cfg.APIKey)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	original := &Config{
		APIKey:      "tuk-test123",
		EndpointURL: "https://custom.example.com",
	}
	if err := Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file permissions
	path := filepath.Join(tmp, ".tilde", "config.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	// Verify directory permissions
	dirInfo, err := os.Stat(filepath.Join(tmp, ".tilde"))
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0700 {
		t.Errorf("dir permissions = %o, want 0700", perm)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.APIKey != original.APIKey {
		t.Errorf("APIKey = %q, want %q", loaded.APIKey, original.APIKey)
	}
	if loaded.EndpointURL != original.EndpointURL {
		t.Errorf("EndpointURL = %q, want %q", loaded.EndpointURL, original.EndpointURL)
	}
}

func TestSaveOmitsEmptyEndpoint(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := &Config{APIKey: "tuk-abc"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(tmp, ".tilde", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if contains := "endpoint_url"; len(content) > 0 {
		for _, line := range splitLines(content) {
			if len(line) > 0 && line[0] != '#' && containsStr(line, contains) {
				t.Errorf("config should not contain endpoint_url when empty, got: %s", content)
			}
		}
	}
}

func TestRemove(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Save then remove
	if err := Save(&Config{APIKey: "tuk-x"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Load should return empty config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load after remove: %v", err)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty after remove", cfg.APIKey)
	}
}

func TestRemoveNonExistent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Should not error when file doesn't exist
	if err := Remove(); err != nil {
		t.Fatalf("Remove non-existent: %v", err)
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findStr(s, substr))
}

func findStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

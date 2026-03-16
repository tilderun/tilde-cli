package cmd

import (
	"testing"
)

func TestParseEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantLen int
		wantErr bool
	}{
		{name: "nil input", input: nil, wantLen: 0},
		{name: "empty input", input: []string{}, wantLen: 0},
		{name: "single var", input: []string{"FOO=bar"}, wantLen: 1},
		{name: "multiple vars", input: []string{"A=1", "B=2"}, wantLen: 2},
		{name: "empty value", input: []string{"KEY="}, wantLen: 1},
		{name: "value with equals", input: []string{"KEY=a=b=c"}, wantLen: 1},
		{name: "missing equals", input: []string{"NOEQUALS"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEnvVars(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseEnvVars() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("parseEnvVars() returned %d entries, want %d", len(got), tt.wantLen)
			}
		})
	}

	// Verify specific values
	m, _ := parseEnvVars([]string{"KEY=a=b=c"})
	if m["KEY"] != "a=b=c" {
		t.Errorf("value = %q, want %q", m["KEY"], "a=b=c")
	}
}

func TestParseDurationToSeconds(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "empty", input: "", want: 0},
		{name: "30 seconds", input: "30s", want: 30},
		{name: "5 minutes", input: "5m", want: 300},
		{name: "1 hour", input: "1h", want: 3600},
		{name: "1.5 minutes", input: "1m30s", want: 90},
		{name: "negative", input: "-5s", wantErr: true},
		{name: "zero", input: "0s", wantErr: true},
		{name: "invalid", input: "notaduration", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDurationToSeconds(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDurationToSeconds(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseDurationToSeconds(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

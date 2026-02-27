package uri

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Parsed
		wantErr bool
	}{
		{
			name:  "org and repo only",
			input: "cb://myorg/myrepo",
			want:  Parsed{Org: "myorg", Repo: "myrepo"},
		},
		{
			name:  "with simple path",
			input: "cb://myorg/myrepo/file.txt",
			want:  Parsed{Org: "myorg", Repo: "myrepo", Path: "file.txt"},
		},
		{
			name:  "with nested path",
			input: "cb://myorg/myrepo/dir/subdir/file.txt",
			want:  Parsed{Org: "myorg", Repo: "myrepo", Path: "dir/subdir/file.txt"},
		},
		{
			name:  "path with trailing slash",
			input: "cb://myorg/myrepo/dir/",
			want:  Parsed{Org: "myorg", Repo: "myrepo", Path: "dir/"},
		},
		{
			name:  "path is just slash",
			input: "cb://myorg/myrepo/",
			want:  Parsed{Org: "myorg", Repo: "myrepo", Path: ""},
		},
		{
			name:  "hyphenated org and repo",
			input: "cb://my-org/my-repo/data",
			want:  Parsed{Org: "my-org", Repo: "my-repo", Path: "data"},
		},

		// Error cases
		{
			name:    "wrong scheme",
			input:   "s3://bucket/key",
			wantErr: true,
		},
		{
			name:    "no scheme",
			input:   "myorg/myrepo/file.txt",
			wantErr: true,
		},
		{
			name:    "empty after scheme",
			input:   "cb://",
			wantErr: true,
		},
		{
			name:    "org only, no repo",
			input:   "cb://myorg",
			wantErr: true,
		},
		{
			name:    "empty org",
			input:   "cb:///myrepo",
			wantErr: true,
		},
		{
			name:    "empty repo",
			input:   "cb://myorg/",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got %+v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}
			if got.Org != tt.want.Org {
				t.Errorf("Org = %q, want %q", got.Org, tt.want.Org)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.want.Repo)
			}
			if got.Path != tt.want.Path {
				t.Errorf("Path = %q, want %q", got.Path, tt.want.Path)
			}
		})
	}
}

func TestIsURI(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"cb://org/repo", true},
		{"cb://org/repo/path", true},
		{"cb://", true},
		{"s3://bucket/key", false},
		{"/local/path", false},
		{"./relative", false},
		{"", false},
		{"-", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsURI(tt.input)
			if got != tt.want {
				t.Errorf("IsURI(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsed_String(t *testing.T) {
	tests := []struct {
		name  string
		input Parsed
		want  string
	}{
		{
			name:  "no path",
			input: Parsed{Org: "org", Repo: "repo"},
			want:  "cb://org/repo",
		},
		{
			name:  "with path",
			input: Parsed{Org: "org", Repo: "repo", Path: "dir/file.txt"},
			want:  "cb://org/repo/dir/file.txt",
		},
		{
			name:  "path with trailing slash",
			input: Parsed{Org: "org", Repo: "repo", Path: "dir/"},
			want:  "cb://org/repo/dir/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRoundTrip(t *testing.T) {
	inputs := []string{
		"cb://org/repo",
		"cb://org/repo/file.txt",
		"cb://org/repo/a/b/c/d.dat",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			parsed, err := Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", input, err)
			}
			got := parsed.String()
			if got != input {
				t.Errorf("round-trip: got %q, want %q", got, input)
			}
		})
	}
}

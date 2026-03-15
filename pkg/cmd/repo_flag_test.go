package cmd

import "testing"

func TestParseRepoFlag(t *testing.T) {
	tests := []struct {
		input   string
		wantOrg string
		wantRepo string
		wantErr bool
	}{
		{"acme/data", "acme", "data", false},
		{"my-org/my-repo", "my-org", "my-repo", false},
		{"a/b", "a", "b", false},
		{"", "", "", true},
		{"noslash", "", "", true},
		{"/repo", "", "", true},
		{"org/", "", "", true},
		{"/", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			org, repo, err := parseRepoFlag(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepoFlag(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if org != tt.wantOrg {
				t.Errorf("org = %q, want %q", org, tt.wantOrg)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

package uri

import (
	"fmt"
	"strings"
)

const Scheme = "cb://"

// Parsed represents a parsed cb://org/repo[/path] URI.
type Parsed struct {
	Org  string
	Repo string
	Path string // may be empty
}

// Parse parses a cb:// URI into its components.
// Format: cb://org/repo[/path...]
func Parse(raw string) (Parsed, error) {
	if !strings.HasPrefix(raw, Scheme) {
		return Parsed{}, fmt.Errorf("invalid URI %q: must start with %s", raw, Scheme)
	}
	rest := strings.TrimPrefix(raw, Scheme)
	if rest == "" {
		return Parsed{}, fmt.Errorf("invalid URI %q: missing organization and repository", raw)
	}

	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Parsed{}, fmt.Errorf("invalid URI %q: must be %sorg/repo[/path]", raw, Scheme)
	}

	p := Parsed{
		Org:  parts[0],
		Repo: parts[1],
	}
	if len(parts) == 3 {
		p.Path = parts[2]
	}
	return p, nil
}

// IsURI returns true if s looks like a cb:// URI.
func IsURI(s string) bool {
	return strings.HasPrefix(s, Scheme)
}

// String returns the URI string representation.
func (p Parsed) String() string {
	if p.Path == "" {
		return fmt.Sprintf("%s%s/%s", Scheme, p.Org, p.Repo)
	}
	return fmt.Sprintf("%s%s/%s/%s", Scheme, p.Org, p.Repo, p.Path)
}

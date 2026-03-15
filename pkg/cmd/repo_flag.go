package cmd

import (
	"fmt"
	"strings"
)

// parseRepoFlag splits "org/repo" into its components.
func parseRepoFlag(s string) (org, repo string, err error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository %q: expected format organization/repository", s)
	}
	return parts[0], parts[1], nil
}

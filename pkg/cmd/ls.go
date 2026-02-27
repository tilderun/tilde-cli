package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
	"github.com/cerebral-storage/cerebral-cli/pkg/uri"
	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	var (
		sessionID  string
		recursive  bool
		maxResults int
	)

	cmd := &cobra.Command{
		Use:   "ls cb://organization/repository[/prefix]",
		Short: "List objects in a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := uri.Parse(args[0])
			if err != nil {
				return err
			}
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}

			prefix := u.Path
			if prefix != "" && !strings.HasSuffix(prefix, "/") {
				prefix += "/"
			}

			params := api.ListObjectsParams{
				SessionID: sessionID,
				Prefix:    prefix,
				Amount:    1000,
			}
			if !recursive {
				params.Delimiter = "/"
			}

			printed := 0
			for {
				resp, err := apiClient.ListObjects(cmd.Context(), u.Org, u.Repo, params)
				if err != nil {
					return err
				}

				for _, entry := range resp.Results {
					if maxResults > 0 && printed >= maxResults {
						return nil
					}
					printListingEntry(entry, prefix)
					printed++
				}

				if !resp.Pagination.HasMore {
					break
				}
				params.After = resp.Pagination.NextOffset
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID (required)")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "List recursively (no delimiter)")
	cmd.Flags().IntVar(&maxResults, "max-results", 0, "Maximum number of results (0=unlimited)")
	return cmd
}

func printListingEntry(e api.ListingEntry, prefix string) {
	displayPath := strings.TrimPrefix(e.Path, prefix)

	// Object lines: "2006-01-02T15:04:05Z  <10-char size>  path"
	// PRE lines:    "                           PRE  path"
	// Both formats align the path column at position 34.
	if e.Type == "prefix" {
		fmt.Printf("%32s  %s\n", "PRE", displayPath)
		return
	}

	var size int64
	var modified string
	if e.Entry != nil {
		size = e.Entry.Size
		if !e.Entry.LastModified.IsZero() {
			modified = e.Entry.LastModified.Format(time.RFC3339)
		}
	}
	if modified == "" {
		modified = strings.Repeat(" ", 20)
	}
	fmt.Printf("%s  %10d  %s\n", modified, size, displayPath)
}

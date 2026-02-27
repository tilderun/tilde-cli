package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
	"github.com/cerebral-storage/cerebral-cli/pkg/uri"
	"github.com/spf13/cobra"
)

const bulkDeleteBatchSize = 1000

func newRmCmd() *cobra.Command {
	var (
		sessionID string
		recursive bool
	)

	cmd := &cobra.Command{
		Use:   "rm --session ID [--recursive] cb://organization/repository/path",
		Short: "Delete objects from a Cerebral repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}

			u, err := uri.Parse(args[0])
			if err != nil {
				return err
			}
			if u.Path == "" {
				return fmt.Errorf("object path is required (cb://organization/repository/path)")
			}

			if recursive {
				return runRecursiveDelete(cmd, u, sessionID)
			}

			return apiClient.DeleteObject(cmd.Context(), u.Org, u.Repo, u.Path, sessionID)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID (required)")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Delete recursively")
	return cmd
}

func runRecursiveDelete(cmd *cobra.Command, u uri.Parsed, sessionID string) error {
	prefix := u.Path
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Collect all object paths under prefix
	var allPaths []string
	params := api.ListObjectsParams{
		SessionID: sessionID,
		Prefix:    prefix,
		Amount:    1000,
	}
	for {
		resp, err := apiClient.ListObjects(cmd.Context(), u.Org, u.Repo, params)
		if err != nil {
			return fmt.Errorf("listing objects: %w", err)
		}
		for _, entry := range resp.Results {
			if entry.Type != "prefix" {
				allPaths = append(allPaths, entry.Path)
			}
		}
		if !resp.Pagination.HasMore {
			break
		}
		params.After = resp.Pagination.NextOffset
	}

	if len(allPaths) == 0 {
		fmt.Fprintf(os.Stderr, "No objects found under %s\n", u.String())
		return nil
	}

	// Delete in batches of 1000
	totalDeleted := 0
	for i := 0; i < len(allPaths); i += bulkDeleteBatchSize {
		end := i + bulkDeleteBatchSize
		if end > len(allPaths) {
			end = len(allPaths)
		}
		batch := allPaths[i:end]

		resp, err := apiClient.BulkDeleteObjects(cmd.Context(), u.Org, u.Repo, sessionID, batch)
		if err != nil {
			return fmt.Errorf("bulk delete failed at batch %d: %w", i/bulkDeleteBatchSize+1, err)
		}
		totalDeleted += resp.Deleted
	}

	fmt.Fprintf(os.Stderr, "Deleted %d objects.\n", totalDeleted)
	return nil
}

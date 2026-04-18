package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// snapshotMaxAttempts bounds retries for one-shot log fetches. Three
// attempts is enough to paper over a transient network blip without
// masking a real outage.
const snapshotMaxAttempts = 3

func newSandboxLogsCmd() *cobra.Command {
	var (
		repoFlag string
		follow   bool
	)

	cmd := &cobra.Command{
		Use:   "logs [flags] <sandbox-id>",
		Short: "Stream sandbox output",
		Long:  "Stream the combined stdout/stderr output of a sandbox.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("missing required argument: <sandbox-id>\n\nUsage: tilde sandbox logs -r <organization>/<repository> <sandbox-id>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, repo, err := parseRepoFlag(repoFlag)
			if err != nil {
				return err
			}
			sandboxID := args[0]

			if follow {
				return streamCombinedWithRetry(cmd.Context(), org, repo, sandboxID, os.Stdout)
			}
			return snapshotLogsWithRetry(cmd.Context(), org, repo, sandboxID, os.Stdout)
		},
	}

	cmd.Flags().StringVarP(&repoFlag, "repository", "r", "", "Repository (organization/repository)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	_ = cmd.MarkFlagRequired("repository")

	return cmd
}

// snapshotLogsWithRetry fetches the current combined-output snapshot of a
// sandbox. It first waits for the sandbox to be past the `starting` state
// (logs do not exist yet). A small number of retries guards against
// transient network failures during the fetch.
func snapshotLogsWithRetry(ctx context.Context, org, repo, sandboxID string, dst io.Writer) error {
	if _, err := waitForLogsAvailable(ctx, org, repo, sandboxID); err != nil {
		return err
	}
	backoff := sandboxReconnectInitialBackoff
	var lastErr error
	for attempt := 0; attempt < snapshotMaxAttempts; attempt++ {
		rc, err := apiClient.GetSandboxOutput(ctx, org, repo, sandboxID, "stdout")
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !sleepWithContext(ctx, backoff) {
				return ctx.Err()
			}
			backoff = nextBackoff(backoff)
			continue
		}
		_, copyErr := io.Copy(dst, rc)
		rc.Close()
		if copyErr == nil {
			return nil
		}
		lastErr = copyErr
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !sleepWithContext(ctx, backoff) {
			return ctx.Err()
		}
		backoff = nextBackoff(backoff)
	}
	return lastErr
}

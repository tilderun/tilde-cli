package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

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
			_ = follow // follow behavior is default — server streams until sandbox finishes
			org, repo, err := parseRepoFlag(repoFlag)
			if err != nil {
				return err
			}
			sandboxID := args[0]

			rc, err := apiClient.StreamSandboxOutput(cmd.Context(), org, repo, sandboxID, "combined")
			if err != nil {
				return err
			}
			defer rc.Close()

			_, _ = io.Copy(os.Stdout, rc)
			return nil
		},
	}

	cmd.Flags().StringVarP(&repoFlag, "repository", "r", "", "Repository (organization/repository)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (default behavior)")
	_ = cmd.MarkFlagRequired("repository")

	return cmd
}

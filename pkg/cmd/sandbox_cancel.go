package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSandboxCancelCmd() *cobra.Command {
	var repoFlag string

	cmd := &cobra.Command{
		Use:   "cancel [flags] <sandbox-id>",
		Short: "Cancel a running sandbox",
		Long:  "Cancel a running sandbox. The sandbox will be stopped and marked as cancelled.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("missing required argument: <sandbox-id>\n\nUsage: tilde sandbox cancel -r <organization>/<repository> <sandbox-id>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, repo, err := parseRepoFlag(repoFlag)
			if err != nil {
				return err
			}
			sandboxID := args[0]

			if err := apiClient.CancelSandbox(cmd.Context(), org, repo, sandboxID); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Sandbox %s cancelled\n", sandboxID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&repoFlag, "repository", "r", "", "Repository (organization/repository)")
	_ = cmd.MarkFlagRequired("repository")

	return cmd
}

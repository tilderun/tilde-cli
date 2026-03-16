package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSandboxInfoCmd() *cobra.Command {
	var repoFlag string

	cmd := &cobra.Command{
		Use:   "info [flags] <sandbox-id>",
		Short: "Show sandbox details",
		Long:  "Display detailed information about a sandbox.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("missing required argument: <sandbox-id>\n\nUsage: tilde sandbox info -r <organization>/<repository> <sandbox-id>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, repo, err := parseRepoFlag(repoFlag)
			if err != nil {
				return err
			}
			sandboxID := args[0]

			sb, err := apiClient.GetSandbox(cmd.Context(), org, repo, sandboxID)
			if err != nil {
				return err
			}

			fmt.Printf("ID:          %s\n", sb.ID)
			fmt.Printf("Status:      %s\n", sb.Status)
			if sb.StatusReason != "" {
				fmt.Printf("Reason:      %s\n", sb.StatusReason)
			}
			if sb.ExitCode != nil {
				fmt.Printf("Exit Code:   %d\n", *sb.ExitCode)
			}
			fmt.Printf("Image:       %s\n", sb.Image)
			if len(sb.Command) > 0 {
				fmt.Printf("Command:     %s\n", strings.Join(sb.Command, " "))
			}
			if sb.CommitID != "" {
				fmt.Printf("Commit ID:   %s\n", sb.CommitID)
			}
			fmt.Printf("Created At:  %s\n", sb.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
			if sb.FinishedAt != nil {
				fmt.Printf("Finished At: %s\n", sb.FinishedAt.Format("2006-01-02 15:04:05 UTC"))
			}
			if sb.ErrorMessage != "" {
				fmt.Printf("Error:       %s\n", sb.ErrorMessage)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&repoFlag, "repository", "r", "", "Repository (organization/repository)")
	_ = cmd.MarkFlagRequired("repository")

	return cmd
}

package cmd

import (
	"fmt"
	"net/url"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
	"github.com/cerebral-storage/cerebral-cli/pkg/uri"
	"github.com/spf13/cobra"
)

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage sessions (start, commit, rollback)",
	}
	cmd.AddCommand(newSessionStartCmd())
	cmd.AddCommand(newSessionCommitCmd())
	cmd.AddCommand(newSessionRollbackCmd())
	return cmd
}

func newSessionStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start cb://organization/repository",
		Short: "Start a new session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := uri.Parse(args[0])
			if err != nil {
				return err
			}
			resp, err := apiClient.CreateSession(cmd.Context(), u.Org, u.Repo)
			if err != nil {
				return err
			}
			fmt.Println(resp.SessionID)
			return nil
		},
	}
}

func newSessionCommitCmd() *cobra.Command {
	var (
		sessionID string
		message   string
		metadata  map[string]string
	)

	cmd := &cobra.Command{
		Use:   "commit cb://organization/repository",
		Short: "Commit a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := uri.Parse(args[0])
			if err != nil {
				return err
			}
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			if message == "" {
				return fmt.Errorf("-m (message) is required")
			}

			resp, err := apiClient.CommitSession(cmd.Context(), u.Org, u.Repo, sessionID, &api.CommitRequest{
				Message:  message,
				Metadata: metadata,
			})
			if err != nil {
				return err
			}

			if resp.ApprovalRequired {
				fmt.Println("Committing this change requires approval from a human.")
				if resp.WebURL != "" {
					parsed, err := url.Parse(resp.WebURL)
					if err == nil {
						q := parsed.Query()
						q.Set("message", message)
						parsed.RawQuery = q.Encode()
						fmt.Printf("Please visit: %s in order to approve this change.\n", parsed.String())
					}
				}
				return nil
			}

			fmt.Println(resp.CommitID)
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID (required)")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message (required)")
	cmd.Flags().StringToStringVar(&metadata, "metadata", nil, "Commit metadata (key=value pairs)")
	return cmd
}

func newSessionRollbackCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "rollback cb://organization/repository",
		Short: "Rollback (discard) a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := uri.Parse(args[0])
			if err != nil {
				return err
			}
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}

			if err := apiClient.RollbackSession(cmd.Context(), u.Org, u.Repo, sessionID); err != nil {
				return err
			}

			fmt.Println("Session rolled back successfully.")
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID (required)")
	return cmd
}

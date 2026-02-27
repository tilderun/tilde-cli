package cmd

import (
	"fmt"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
	"github.com/spf13/cobra"
)

func newRepositoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repository",
		Short: "Manage and list repositories",
	}
	cmd.AddCommand(newRepositoryLsCmd())
	return cmd
}

func newRepositoryLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls [organization]",
		Short: "List repositories",
		Long:  "List repositories accessible to the caller. If an organization name is provided, only repositories in that organization are listed.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var org string
			if len(args) == 1 {
				org = args[0]
			}

			params := api.ListRepositoriesParams{
				Amount: 1000,
			}

			for {
				resp, err := apiClient.ListRepositories(cmd.Context(), org, params)
				if err != nil {
					return err
				}

				for _, repo := range resp.Results {
					fmt.Printf("%s/%s\n", repo.OrganizationSlug, repo.Name)
				}

				if !resp.Pagination.HasMore {
					break
				}
				params.After = resp.Pagination.NextOffset
			}
			return nil
		},
	}
}

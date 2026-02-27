package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
	"github.com/spf13/cobra"
)

const (
	defaultEndpoint    = "https://cerebral.storage"
	defaultConcurrency = 16
	apiKeyPrefix       = "cak-"
)

// Global state shared across subcommands
var (
	apiClient      *api.Client
	maxConcurrency int
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "cerebral",
		Short: "CLI for the Cerebral data versioning API",
		Long: `cerebral is a command-line tool for managing data objects in Cerebral repositories
using session-based workflows.

Repositories are referenced using the cb:// URI scheme:

  cb://organization/repository[/path]

All data operations (cp, ls, rm) require an active session. Create one with:

  cerebral session start cb://organization/repository

Then pass the returned session ID to every command via --session.

Changes made within a session are staged — they are not visible to other sessions
or durably stored until committed:

  cerebral session commit --session SESSION_ID -m "description" cb://organization/repository

To discard uncommitted changes, roll back the session:

  cerebral session rollback --session SESSION_ID cb://organization/repository`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip validation for help and completion commands
			if cmd.Name() == "help" || cmd.Name() == "completion" {
				return nil
			}

			apiKey := os.Getenv("CEREBRAL_API_KEY")
			if apiKey == "" {
				return fmt.Errorf("CEREBRAL_API_KEY environment variable is required.\nSet it to your agent API key (starts with %q).", apiKeyPrefix)
			}
			if !strings.HasPrefix(apiKey, apiKeyPrefix) {
				return fmt.Errorf("CEREBRAL_API_KEY must start with %q. Got: %q...", apiKeyPrefix, apiKey[:min(len(apiKey), 8)])
			}

			endpoint := os.Getenv("CEREBRAL_ENDPOINT_URL")
			if endpoint == "" {
				endpoint = defaultEndpoint
			}
			endpoint = strings.TrimRight(endpoint, "/")
			baseURL := endpoint + "/api/v1"

			apiClient = api.NewClient(baseURL, apiKey)

			// Parse concurrency
			maxConcurrency = defaultConcurrency
			if v := os.Getenv("CEREBRAL_CLI_MAX_CONCURRENCY"); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil || n < 1 {
					return fmt.Errorf("CEREBRAL_CLI_MAX_CONCURRENCY must be a positive integer, got %q", v)
				}
				maxConcurrency = n
			}

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newSessionCmd())
	root.AddCommand(newRepositoryCmd())
	root.AddCommand(newCpCmd())
	root.AddCommand(newRmCmd())
	root.AddCommand(newLsCmd())

	return root
}

// Execute is the main entry point for the CLI.
func Execute() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	root := NewRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

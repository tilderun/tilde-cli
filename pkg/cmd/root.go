package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/tilderun/tilde-cli/pkg/api"
	"github.com/spf13/cobra"
)

const defaultEndpoint = "https://tilde.run"

var validKeyPrefixes = []string{"tuk-", "trk-", "tak-"}

// Global state shared across subcommands
var apiClient *api.Client

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "tilde",
		Short: "CLI for the Tilde sandbox runtime",
		Long: `tilde is a command-line tool for running sandboxed workloads on Tilde.

Run a sandbox:

  tilde sandbox run -r organization/repository --image alpine -- echo hello

Get an interactive shell:

  tilde shell organization/repository

Execute a command:

  tilde exec organization/repository -- ls -la`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip validation for help and completion commands
			if cmd.Name() == "help" || cmd.Name() == "completion" {
				return nil
			}

			apiKey := os.Getenv("TILDE_API_KEY")
			if apiKey == "" {
				return fmt.Errorf("TILDE_API_KEY environment variable is required.\nSet it to your API key (starts with %q).", validKeyPrefixes[0])
			}
			validPrefix := false
			for _, p := range validKeyPrefixes {
				if strings.HasPrefix(apiKey, p) {
					validPrefix = true
					break
				}
			}
			if !validPrefix {
				return fmt.Errorf("TILDE_API_KEY must start with one of %v. Got: %q...", validKeyPrefixes, apiKey[:min(len(apiKey), 8)])
			}

			endpoint := os.Getenv("TILDE_ENDPOINT_URL")
			if endpoint == "" {
				endpoint = defaultEndpoint
			}
			endpoint = strings.TrimRight(endpoint, "/")
			baseURL := endpoint + "/api/v1"

			apiClient = api.NewClient(baseURL, apiKey)

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newSandboxCmd())
	root.AddCommand(newShellCmd())
	root.AddCommand(newExecCmd())
	root.AddCommand(newRepositoryCmd())

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

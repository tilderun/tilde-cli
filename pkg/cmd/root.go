package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tilderun/tilde-cli/pkg/api"
	"github.com/tilderun/tilde-cli/pkg/config"
)

const defaultEndpoint = "https://tilde.run"

var validKeyPrefixes = []string{"tuk-", "trk-", "tak-"}

// Global state shared across subcommands
var apiClient *api.Client

// resolveAPIKey returns the API key and endpoint using the precedence:
// CLI flag > env var > config file, and endpoint from env > config > default.
func resolveAPIKey() (apiKey, endpoint string) {
	endpoint = resolveEndpoint()
	// CLI flag is injected by the caller before this is used.
	apiKey = os.Getenv("TILDE_API_KEY")
	if apiKey != "" {
		return apiKey, endpoint
	}
	cfg, err := config.Load()
	if err == nil && cfg.APIKey != "" {
		apiKey = cfg.APIKey
	}
	return apiKey, endpoint
}

// resolveEndpoint returns the endpoint using precedence: env > config > default.
func resolveEndpoint() string {
	if ep := os.Getenv("TILDE_ENDPOINT_URL"); ep != "" {
		return ep
	}
	cfg, err := config.Load()
	if err == nil && cfg.EndpointURL != "" {
		return cfg.EndpointURL
	}
	return defaultEndpoint
}

func NewRootCmd() *cobra.Command {
	var flagAPIKey string

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
			// Skip validation for help, completion, and auth commands
			if cmd.Name() == "help" || cmd.Name() == "completion" {
				return nil
			}
			// Skip for auth subcommands — they handle credentials themselves
			if cmd.Parent() != nil && cmd.Parent().Name() == "auth" {
				return nil
			}

			// Resolve API key: flag > env > config
			apiKey := flagAPIKey
			if apiKey == "" {
				apiKey = os.Getenv("TILDE_API_KEY")
			}
			if apiKey == "" {
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("loading config: %w", err)
				}
				apiKey = cfg.APIKey
			}

			if apiKey == "" {
				return fmt.Errorf("no API key found.\nRun \"tilde auth login\" to authenticate, set TILDE_API_KEY, or pass --api-key.")
			}
			validPrefix := false
			for _, p := range validKeyPrefixes {
				if strings.HasPrefix(apiKey, p) {
					validPrefix = true
					break
				}
			}
			if !validPrefix {
				return fmt.Errorf("API key must start with one of %v. Got: %q...", validKeyPrefixes, apiKey[:min(len(apiKey), 8)])
			}

			endpoint := resolveEndpoint()
			endpoint = strings.TrimRight(endpoint, "/")
			baseURL := endpoint + "/api/v1"

			apiClient = api.NewClient(baseURL, apiKey)

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key (overrides TILDE_API_KEY and config file)")

	root.AddCommand(newSandboxCmd())
	root.AddCommand(newShellCmd())
	root.AddCommand(newExecCmd())
	root.AddCommand(newRepositoryCmd())
	root.AddCommand(newAuthCmd())

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

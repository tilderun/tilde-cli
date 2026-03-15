package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tilderun/tilde-cli/pkg/api"
	"github.com/spf13/cobra"
)

const defaultImage = "busybox:latest"

func newShellCmd() *cobra.Command {
	var (
		image   string
		envVars []string
		timeout string
	)

	cmd := &cobra.Command{
		Use:   "shell <organization>/<repository> [-- COMMAND...]",
		Short: "Start an interactive shell in a sandbox",
		Long:  "Creates an interactive sandbox and attaches a terminal.\nOptionally pass a command after -- to run interactively instead of the default shell.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("missing required argument: <organization>/<repository>\n\nUsage: tilde shell <organization>/<repository> [-- COMMAND...]")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, repo, err := parseRepoFlag(args[0])
			if err != nil {
				return err
			}

			timeoutSeconds, err := parseDurationToSeconds(timeout)
			if err != nil {
				return err
			}

			envMap, err := parseEnvVars(envVars)
			if err != nil {
				return err
			}

			req := api.CreateSandboxRequest{
				Image:          image,
				Command:        args[1:], // empty if no command provided
				Interactive:    true,
				TimeoutSeconds: timeoutSeconds,
				EnvVars:        envMap,
			}

			resp, err := apiClient.CreateSandbox(cmd.Context(), org, repo, req)
			if err != nil {
				return err
			}

			if err := waitForRunning(cmd.Context(), org, repo, resp.SandboxID); err != nil {
				return err
			}

			wsURL := apiClient.TerminalWebSocketURL(org, repo, resp.SandboxID)
			exitCode, err := attachTerminal(cmd.Context(), wsURL, apiClient.APIKey)
			if err != nil {
				return err
			}
			os.Exit(exitCode)
			return nil
		},
	}

	cmd.Flags().StringVar(&image, "image", defaultImage, "Container image")
	cmd.Flags().StringArrayVarP(&envVars, "env", "e", nil, "Environment variables (KEY=VALUE)")
	cmd.Flags().StringVar(&timeout, "timeout", "", "Sandbox timeout (e.g. 30s, 5m, 1h)")

	return cmd
}

// parseEnvVars parses KEY=VALUE pairs into a map.
func parseEnvVars(envVars []string) (map[string]string, error) {
	if len(envVars) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(envVars))
	for _, e := range envVars {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			return nil, fmt.Errorf("invalid env var %q: expected KEY=VALUE", e)
		}
		m[k] = v
	}
	return m, nil
}

// parseDurationToSeconds parses a Go duration string and returns whole seconds.
// Returns 0 if the input is empty.
func parseDurationToSeconds(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("timeout must be positive, got %s", s)
	}
	return int(d.Seconds()), nil
}

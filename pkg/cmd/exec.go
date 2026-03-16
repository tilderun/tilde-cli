package cmd

import (
	"fmt"

	"github.com/tilderun/tilde-cli/pkg/api"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var (
		image   string
		envVars []string
		timeout string
	)

	cmd := &cobra.Command{
		Use:   "exec <organization>/<repository> -- COMMAND...",
		Short: "Execute a command in a sandbox",
		Long:  "Creates a non-interactive sandbox, runs the given command, streams output, and exits with the sandbox's exit code.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("missing required arguments: <organization>/<repository> and command\n\nUsage: tilde exec <organization>/<repository> -- COMMAND...")
			}
			if len(args) < 2 {
				return fmt.Errorf("missing command after <organization>/<repository>\n\nUsage: tilde exec <organization>/<repository> -- COMMAND...")
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

			command := args[1:]

			req := api.CreateSandboxRequest{
				Image:          image,
				Command:        command,
				TimeoutSeconds: timeoutSeconds,
				EnvVars:        envMap,
			}

			return runAndStream(cmd, org, repo, req)
		},
	}

	cmd.Flags().StringVar(&image, "image", defaultImage, "Container image")
	cmd.Flags().StringArrayVarP(&envVars, "env", "e", nil, "Environment variables (KEY=VALUE)")
	cmd.Flags().StringVar(&timeout, "timeout", "", "Sandbox timeout (e.g. 30s, 5m, 1h)")

	return cmd
}

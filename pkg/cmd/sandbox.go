package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tilderun/tilde-cli/pkg/api"
	"github.com/spf13/cobra"
)

func newSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage sandboxes",
	}
	cmd.AddCommand(newSandboxRunCmd())
	cmd.AddCommand(newSandboxLogsCmd())
	cmd.AddCommand(newSandboxInfoCmd())
	return cmd
}

func newSandboxRunCmd() *cobra.Command {
	var (
		repoFlag    string
		image       string
		timeout     int
		envVars     []string
		interactive bool
		detach      bool
		mountpoint  string
		pathPrefix  string
	)

	cmd := &cobra.Command{
		Use:   "run [flags] [-- COMMAND...]",
		Short: "Create and run a sandbox",
		Long: `Create a new sandbox and run it. By default, streams combined output to stdout
and exits with the sandbox's exit code.

Use -d/--detach to create the sandbox and print its ID without waiting.
Use -i/--interactive to attach an interactive terminal.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			org, repo, err := parseRepoFlag(repoFlag)
			if err != nil {
				return err
			}

			envMap, err := parseEnvVars(envVars)
			if err != nil {
				return err
			}

			req := api.CreateSandboxRequest{
				Image:          image,
				Command:        args,
				Mountpoint:     mountpoint,
				PathPrefix:     pathPrefix,
				TimeoutSeconds: timeout,
				EnvVars:        envMap,
				Interactive:    interactive,
			}

			if detach {
				resp, err := apiClient.CreateSandbox(cmd.Context(), org, repo, req)
				if err != nil {
					return err
				}
				fmt.Println(resp.SandboxID)
				return nil
			}

			if interactive {
				req.Interactive = true
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
			}

			// Default: stream output and wait for exit
			return runAndStream(cmd, org, repo, req)
		},
	}

	cmd.Flags().StringVarP(&repoFlag, "repository", "r", "", "Repository (organization/repository)")
	cmd.Flags().StringVar(&image, "image", "", "Container image")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Timeout in seconds")
	cmd.Flags().StringArrayVarP(&envVars, "env", "e", nil, "Environment variables (KEY=VALUE)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Attach interactive terminal")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Detach after creating sandbox")
	cmd.Flags().StringVar(&mountpoint, "mountpoint", "", "Mount point for repository data")
	cmd.Flags().StringVar(&pathPrefix, "path-prefix", "", "Path prefix for repository data")

	_ = cmd.MarkFlagRequired("repository")
	_ = cmd.MarkFlagRequired("image")

	return cmd
}

// runAndStream creates a sandbox, streams its combined output, then exits with its exit code.
func runAndStream(cmd *cobra.Command, org, repo string, req api.CreateSandboxRequest) error {
	ctx := cmd.Context()

	resp, err := apiClient.CreateSandbox(ctx, org, repo, req)
	if err != nil {
		return err
	}

	rc, err := apiClient.StreamSandboxOutput(ctx, org, repo, resp.SandboxID, "combined")
	if err != nil {
		return fmt.Errorf("streaming output: %w", err)
	}
	_, _ = io.Copy(os.Stdout, rc)
	rc.Close()

	// Poll for final status
	for {
		status, err := apiClient.GetSandboxStatus(ctx, org, repo, resp.SandboxID)
		if err != nil {
			return fmt.Errorf("getting sandbox status: %w", err)
		}
		switch status.Status {
		case "committed", "awaiting_approval", "failed", "cancelled":
			if status.ExitCode != nil {
				os.Exit(*status.ExitCode)
			}
			if status.Status == "failed" {
				os.Exit(1)
			}
			return nil
		}
		time.Sleep(time.Second)
	}
}

// waitForRunning polls sandbox status until it reaches "running" or a terminal state.
func waitForRunning(ctx context.Context, org, repo, sandboxID string) error {
	for {
		status, err := apiClient.GetSandboxStatus(ctx, org, repo, sandboxID)
		if err != nil {
			return fmt.Errorf("waiting for sandbox: %w", err)
		}
		switch status.Status {
		case "running":
			return nil
		case "committed", "awaiting_approval", "failed", "cancelled":
			return fmt.Errorf("sandbox reached terminal state %q before becoming ready", status.Status)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

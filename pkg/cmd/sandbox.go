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

// Bounds for exponential backoff when reconnecting to sandbox streams or
// terminals. Starting at 500ms keeps typical reconnects snappy; capping at
// 10s avoids hammering a struggling server.
const (
	sandboxReconnectInitialBackoff = 500 * time.Millisecond
	sandboxReconnectMaxBackoff     = 10 * time.Second
)

func newSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage sandboxes",
	}
	cmd.AddCommand(newSandboxRunCmd())
	cmd.AddCommand(newSandboxLogsCmd())
	cmd.AddCommand(newSandboxInfoCmd())
	cmd.AddCommand(newSandboxCancelCmd())
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

			if interactive {
				envMap = withInteractiveTerm(envMap)
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
				fmt.Fprintln(cmd.OutOrStdout(), resp.SandboxID)
				return nil
			}

			if interactive {
				resp, err := apiClient.CreateSandbox(cmd.Context(), org, repo, req)
				if err != nil {
					return err
				}
				if _, err := attachTerminalWithReconnect(cmd.Context(), org, repo, resp.SandboxID); err != nil {
					return err
				}
				return waitForFinalStatus(cmd.Context(), org, repo, resp.SandboxID)
			}

			// Default: stream output and wait for exit
			return runAndStream(cmd, org, repo, req)
		},
	}

	cmd.Flags().StringVarP(&repoFlag, "repository", "r", "", "Repository (organization/repository)")
	cmd.Flags().StringVar(&image, "image", "", "Docker image reference (e.g. python:3.12, ubuntu:22.04, my-org/my-image:latest)")
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

	if err := streamCombinedWithRetry(ctx, org, repo, resp.SandboxID, os.Stdout); err != nil {
		return fmt.Errorf("streaming output: %w", err)
	}

	return waitForFinalStatus(ctx, org, repo, resp.SandboxID)
}

// attachTerminalWithReconnect attaches to a sandbox's interactive terminal,
// waiting for the sandbox to be ready first and reconnecting if the
// WebSocket drops while the sandbox is still running. Returns the exit code
// from the sandbox when the server sends an exit frame, or 0 if the session
// ended without one (e.g. user disconnected cleanly).
//
// Dial failures are returned as-is rather than retried: the caller has
// already confirmed the sandbox is `running`, so a failed dial indicates a
// persistent condition (auth, server error, non-interactive sandbox) that
// retry won't fix. Only mid-session drops trigger a reconnect.
func attachTerminalWithReconnect(ctx context.Context, org, repo, sandboxID string) (int, error) {
	backoff := sandboxReconnectInitialBackoff
	for {
		if err := waitForRunning(ctx, org, repo, sandboxID); err != nil {
			return 0, err
		}
		wsURL := apiClient.TerminalWebSocketURL(org, repo, sandboxID)
		token, err := apiClient.Token(ctx)
		if err != nil {
			return 0, err
		}
		exitCode, dropped, err := attachTerminal(ctx, wsURL, token)
		if err != nil {
			return exitCode, err
		}
		if !dropped {
			// Clean exit (exit frame or user-initiated close).
			return exitCode, nil
		}
		// Connection dropped: reconnect only if the sandbox is still
		// active. If it has reached a terminal state we have nothing to
		// reattach to.
		if ctx.Err() != nil {
			return exitCode, ctx.Err()
		}
		status, sErr := apiClient.GetSandboxStatus(ctx, org, repo, sandboxID)
		if sErr != nil || status.IsTerminal() {
			return exitCode, nil
		}
		if !sleepWithContext(ctx, backoff) {
			return exitCode, ctx.Err()
		}
		backoff = nextBackoff(backoff)
		fmt.Fprintln(os.Stderr, "reconnecting to sandbox terminal...")
	}
}

// streamCombinedWithRetry streams the combined stdout/stderr of a sandbox to
// dst. It waits for the sandbox to leave the `starting` state before
// connecting, and reconnects (with exponential backoff) if the stream drops
// while the sandbox is still active.
func streamCombinedWithRetry(ctx context.Context, org, repo, sandboxID string, dst io.Writer) error {
	backoff := sandboxReconnectInitialBackoff
	for {
		if _, err := waitForLogsAvailable(ctx, org, repo, sandboxID); err != nil {
			return err
		}
		rc, err := apiClient.StreamSandboxOutput(ctx, org, repo, sandboxID, "stdout")
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			status, sErr := apiClient.GetSandboxStatus(ctx, org, repo, sandboxID)
			if sErr == nil && status.IsTerminal() {
				return nil
			}
			if !sleepWithContext(ctx, backoff) {
				return ctx.Err()
			}
			backoff = nextBackoff(backoff)
			continue
		}
		_, _ = io.Copy(dst, rc)
		rc.Close()
		if ctx.Err() != nil {
			return nil
		}
		status, err := apiClient.GetSandboxStatus(ctx, org, repo, sandboxID)
		if err != nil || status.IsTerminal() {
			return nil
		}
		if !sleepWithContext(ctx, backoff) {
			return nil
		}
		backoff = nextBackoff(backoff)
	}
}

// waitForLogsAvailable polls the sandbox status until it has moved past the
// `starting` state, at which point log streams can be read. Returns the
// observed status to help callers branch on terminal vs running.
func waitForLogsAvailable(ctx context.Context, org, repo, sandboxID string) (*api.SandboxStatusResponse, error) {
	for {
		status, err := apiClient.GetSandboxStatus(ctx, org, repo, sandboxID)
		if err != nil {
			return nil, fmt.Errorf("getting sandbox status: %w", err)
		}
		if status.LogsAvailable() {
			return status, nil
		}
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// sleepWithContext sleeps for d or until ctx is done. Returns true if the
// full duration elapsed, false if the context was cancelled.
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// nextBackoff doubles the current backoff up to sandboxReconnectMaxBackoff.
func nextBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > sandboxReconnectMaxBackoff {
		return sandboxReconnectMaxBackoff
	}
	return d
}

// waitForFinalStatus polls until the sandbox reaches a terminal state and
// prints the outcome (commit ID, approval URL, failure reason, etc.).
func waitForFinalStatus(ctx context.Context, org, repo, sandboxID string) error {
	for {
		status, err := apiClient.GetSandboxStatus(ctx, org, repo, sandboxID)
		if err != nil {
			return fmt.Errorf("getting sandbox status: %w", err)
		}
		switch status.Status {
		case api.SandboxStatusCommitted:
			if status.CommitID != "" {
				fmt.Fprintf(os.Stderr, "Committed: %s\n", status.CommitID)
			} else if status.StatusReason == "no_changes" {
				fmt.Fprintln(os.Stderr, "Done (no changes)")
			}
			if status.ExitCode != nil {
				os.Exit(*status.ExitCode)
			}
			return nil
		case api.SandboxStatusAwaitingApproval:
			fmt.Fprintln(os.Stderr, "Commit requires approval")
			if status.WebURL != "" {
				fmt.Fprintf(os.Stderr, "Approve at: %s\n", status.WebURL)
			}
			if status.ExitCode != nil {
				os.Exit(*status.ExitCode)
			}
			return nil
		case api.SandboxStatusFailed:
			if status.ErrorMessage != "" {
				fmt.Fprintf(os.Stderr, "Failed: %s\n", status.ErrorMessage)
			} else if status.StatusReason != "" {
				fmt.Fprintf(os.Stderr, "Failed: %s\n", status.StatusReason)
			}
			if status.ExitCode != nil {
				os.Exit(*status.ExitCode)
			}
			os.Exit(1)
		case api.SandboxStatusCancelled:
			fmt.Fprintln(os.Stderr, "Cancelled")
			if status.ExitCode != nil {
				os.Exit(*status.ExitCode)
			}
			return nil
		}
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// waitForRunning polls sandbox status until it reaches `running`. Returns an
// error if the sandbox hits a terminal state first, since there is nothing
// to attach to at that point.
func waitForRunning(ctx context.Context, org, repo, sandboxID string) error {
	for {
		status, err := apiClient.GetSandboxStatus(ctx, org, repo, sandboxID)
		if err != nil {
			return fmt.Errorf("waiting for sandbox: %w", err)
		}
		if status.IsAttachable() {
			return nil
		}
		if status.IsTerminal() {
			return fmt.Errorf("sandbox reached terminal state %q before becoming ready", status.Status)
		}
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

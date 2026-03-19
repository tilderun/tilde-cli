package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tilderun/tilde-cli/pkg/api"
	"github.com/tilderun/tilde-cli/pkg/config"
)

type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type deviceTokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	Error       string `json:"error,omitempty"`
}

type authMeResponse struct {
	User struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user"`
}

func newAuthCmd() *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}
	auth.AddCommand(newAuthLoginCmd())
	auth.AddCommand(newAuthLogoutCmd())
	auth.AddCommand(newAuthStatusCmd())
	return auth
}

func newAuthLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in with your browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd.Context())
		},
	}
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out and remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Remove(); err != nil {
				return err
			}
			fmt.Println("Logged out.")
			return nil
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(cmd.Context())
		},
	}
}

func runLogin(ctx context.Context) error {
	endpoint := resolveEndpoint()
	baseURL := strings.TrimRight(endpoint, "/") + "/api/v1"

	// Request device code
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/auth/device/code", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d requesting device code", resp.StatusCode)
	}

	var dc deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return fmt.Errorf("decoding device code response: %w", err)
	}

	fmt.Printf("\nYour one-time code: %s\n\n", dc.UserCode)
	fmt.Printf("Press Enter to open %s in your browser...\n", dc.VerificationURIComplete)
	fmt.Scanln()

	// Try to open browser (non-fatal)
	openBrowser(dc.VerificationURIComplete)

	fmt.Println("Waiting for authorization...")

	// Poll for token
	interval := time.Duration(dc.Interval) * time.Second
	if interval == 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("device authorization expired")
		}

		token, err := pollDeviceToken(ctx, client, baseURL, dc.DeviceCode)
		if err != nil {
			return err
		}
		if token == "" {
			continue // authorization_pending
		}

		// Got a token — determine endpoint_url to save
		cfg := &config.Config{APIKey: token}
		if endpoint != defaultEndpoint {
			cfg.EndpointURL = endpoint
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving credentials: %w", err)
		}
		fmt.Println("Login successful!")
		return nil
	}
}

// pollDeviceToken returns the access token, empty string for "pending", or an error.
func pollDeviceToken(ctx context.Context, client *http.Client, baseURL, deviceCode string) (string, error) {
	body := fmt.Sprintf(`{"device_code":%q}`, deviceCode)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/auth/device/token",
		strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("polling for token: %w", err)
	}
	defer resp.Body.Close()

	var dt deviceTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&dt); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	if dt.AccessToken != "" {
		return dt.AccessToken, nil
	}

	switch dt.Error {
	case "authorization_pending":
		return "", nil
	case "slow_down":
		// Caller's interval will naturally space out; we just wait a bit extra
		time.Sleep(1 * time.Second)
		return "", nil
	case "expired_token":
		return "", fmt.Errorf("device authorization expired")
	default:
		return "", fmt.Errorf("unexpected error from auth server: %s", dt.Error)
	}
}

func runAuthStatus(ctx context.Context) error {
	apiKey, endpoint := resolveAPIKey()
	if apiKey == "" {
		fmt.Println("Not logged in.")
		return nil
	}

	baseURL := strings.TrimRight(endpoint, "/") + "/api/v1"
	client := api.NewClient(baseURL, apiKey)

	var me authMeResponse
	_, err := client.DoJSON(ctx, http.MethodGet, "/auth/me", nil, &me)
	if err != nil {
		fmt.Println("Not logged in (invalid or expired token).")
		return nil
	}

	fmt.Printf("Logged in as %s (%s)\n", me.User.Username, me.User.Email)
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start() // non-fatal
}

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// CreateSandbox creates a new sandbox in a repository.
func (c *Client) CreateSandbox(ctx context.Context, org, repo string, req CreateSandboxRequest) (*CreateSandboxResponse, error) {
	path := fmt.Sprintf("/organizations/%s/repositories/%s/sandboxes", url.PathEscape(org), url.PathEscape(repo))
	var resp CreateSandboxResponse
	_, err := c.DoJSON(ctx, http.MethodPost, path, &req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetSandbox retrieves a sandbox by ID.
func (c *Client) GetSandbox(ctx context.Context, org, repo, sandboxID string) (*Sandbox, error) {
	path := fmt.Sprintf("/organizations/%s/repositories/%s/sandboxes/%s",
		url.PathEscape(org), url.PathEscape(repo), url.PathEscape(sandboxID))
	var resp Sandbox
	_, err := c.DoJSON(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelSandbox cancels a running sandbox (expects 202 Accepted).
func (c *Client) CancelSandbox(ctx context.Context, org, repo, sandboxID string) error {
	path := fmt.Sprintf("/organizations/%s/repositories/%s/sandboxes/%s",
		url.PathEscape(org), url.PathEscape(repo), url.PathEscape(sandboxID))
	resp, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}
	return nil
}

// GetSandboxStatus retrieves the status of a sandbox.
func (c *Client) GetSandboxStatus(ctx context.Context, org, repo, sandboxID string) (*SandboxStatusResponse, error) {
	path := fmt.Sprintf("/organizations/%s/repositories/%s/sandboxes/%s/status",
		url.PathEscape(org), url.PathEscape(repo), url.PathEscape(sandboxID))
	var resp SandboxStatusResponse
	_, err := c.DoJSON(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// StreamSandboxOutput streams sandbox output (stdout, stderr, or combined).
// The caller must close the returned ReadCloser.
func (c *Client) StreamSandboxOutput(ctx context.Context, org, repo, sandboxID, stream string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/organizations/%s/repositories/%s/sandboxes/%s/%s",
		url.PathEscape(org), url.PathEscape(repo), url.PathEscape(sandboxID), url.PathEscape(stream))
	resp, err := c.doStream(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// GetSandboxOutput fetches sandbox output as a snapshot using the regular HTTP client (with timeout).
// The caller must close the returned ReadCloser.
func (c *Client) GetSandboxOutput(ctx context.Context, org, repo, sandboxID, stream string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/organizations/%s/repositories/%s/sandboxes/%s/%s",
		url.PathEscape(org), url.PathEscape(repo), url.PathEscape(sandboxID), url.PathEscape(stream))
	resp, err := c.doRaw(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// TerminalWebSocketURL constructs the WebSocket URL for terminal attachment.
func (c *Client) TerminalWebSocketURL(org, repo, sandboxID string) string {
	wsBase := strings.Replace(c.BaseURL, "https://", "wss://", 1)
	wsBase = strings.Replace(wsBase, "http://", "ws://", 1)
	return fmt.Sprintf("%s/organizations/%s/repositories/%s/sandboxes/%s/terminal",
		wsBase, url.PathEscape(org), url.PathEscape(repo), url.PathEscape(sandboxID))
}

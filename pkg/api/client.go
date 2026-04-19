package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the Tilde API HTTP client.
type Client struct {
	BaseURL      string // e.g. "https://tilde.run/api/v1"
	TokenSource  TokenSource
	HTTPClient   *http.Client
	StreamClient *http.Client // separate client for long-lived streaming (no timeout)
}

// NewClient creates a new API client backed by a static API key.
func NewClient(baseURL, apiKey string) *Client {
	return NewClientWithTokenSource(baseURL, StaticTokenSource{Value: apiKey})
}

// NewClientWithTokenSource creates a new API client that fetches bearer
// tokens from the given TokenSource. Used for sandbox-principal tokens
// (see pkg/api/credentials.go) that must be refreshed before expiry.
func NewClientWithTokenSource(baseURL string, src TokenSource) *Client {
	return &Client{
		BaseURL:     baseURL,
		TokenSource: src,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects — we need to capture 307 for presigned URLs
				return http.ErrUseLastResponse
			},
		},
		StreamClient: &http.Client{
			// No timeout for long-lived streaming connections
			// Default redirect policy (follows redirects)
		},
	}
}

// Token returns the current bearer token from the client's token source.
func (c *Client) Token(ctx context.Context) (string, error) {
	if c.TokenSource == nil {
		return "", fmt.Errorf("client has no token source")
	}
	return c.TokenSource.Token(ctx)
}

// do executes an HTTP request against the API with authentication.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	token, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.HTTPClient.Do(req)
}

// doStream executes an HTTP request using the StreamClient (no timeout) for long-lived connections.
func (c *Client) doStream(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	token, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.StreamClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return resp, parseAPIError(resp)
	}
	return resp, nil
}

// DoJSON executes an API request and decodes the JSON response.
func (c *Client) DoJSON(ctx context.Context, method, path string, reqBody, respBody any) (*http.Response, error) {
	var body io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return resp, parseAPIError(resp)
	}

	if respBody != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			return resp, fmt.Errorf("decoding response: %w", err)
		}
	}
	return resp, nil
}

// doRaw executes an API request and returns the raw response (caller must close body).
func (c *Client) doRaw(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	token, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return resp, parseAPIError(resp)
	}
	return resp, nil
}

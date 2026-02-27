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

// Client is the Cerebral API HTTP client.
type Client struct {
	BaseURL    string // e.g. "https://cerebral.storage/api/v1"
	APIKey     string
	HTTPClient *http.Client
	S3Client   *http.Client // separate client for S3 (no auth, follows redirects, no timeout)
}

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects — we need to capture 307 for presigned URLs
				return http.ErrUseLastResponse
			},
		},
		S3Client: &http.Client{
			// No timeout for large file transfers
			// Default redirect policy (follows redirects)
		},
	}
}

// do executes an HTTP request against the API with authentication.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.HTTPClient.Do(req)
}

// doJSON executes an API request and decodes the JSON response.
func (c *Client) doJSON(ctx context.Context, method, path string, reqBody, respBody any) (*http.Response, error) {
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
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
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

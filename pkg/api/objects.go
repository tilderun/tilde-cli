package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// GetObjectPresignedURL returns the presigned S3 download URL for an object.
// It issues a GET with presign=true and captures the 307 redirect Location header.
func (c *Client) GetObjectPresignedURL(ctx context.Context, org, repo, objPath, sessionID string) (string, error) {
	params := url.Values{}
	params.Set("path", objPath)
	params.Set("presign", "true")
	if sessionID != "" {
		params.Set("session_id", sessionID)
	}
	path := fmt.Sprintf("/organizations/%s/repositories/%s/object?%s", org, repo, params.Encode())

	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTemporaryRedirect {
		loc := resp.Header.Get("Location")
		if loc == "" {
			return "", fmt.Errorf("presigned redirect missing Location header")
		}
		return loc, nil
	}

	if resp.StatusCode >= 400 {
		return "", parseAPIError(resp)
	}

	return "", fmt.Errorf("unexpected status %d for presigned GET", resp.StatusCode)
}

// StageObject requests a presigned upload URL for an object.
func (c *Client) StageObject(ctx context.Context, org, repo, objPath, sessionID string) (*StageResponse, error) {
	params := url.Values{}
	params.Set("path", objPath)
	params.Set("session_id", sessionID)
	path := fmt.Sprintf("/organizations/%s/repositories/%s/object/stage?%s", org, repo, params.Encode())

	var resp StageResponse
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// FinalizeObject completes a presigned upload.
func (c *Client) FinalizeObject(ctx context.Context, org, repo, objPath, sessionID string, expiresAt string, req *FinalizeRequest) (*FinalizeResponse, error) {
	params := url.Values{}
	params.Set("path", objPath)
	params.Set("session_id", sessionID)
	params.Set("expires_at", expiresAt)
	path := fmt.Sprintf("/organizations/%s/repositories/%s/object/finalize?%s", org, repo, params.Encode())

	var resp FinalizeResponse
	_, err := c.doJSON(ctx, http.MethodPut, path, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteObject deletes a single object.
func (c *Client) DeleteObject(ctx context.Context, org, repo, objPath, sessionID string) error {
	params := url.Values{}
	params.Set("path", objPath)
	params.Set("session_id", sessionID)
	path := fmt.Sprintf("/organizations/%s/repositories/%s/object?%s", org, repo, params.Encode())

	_, err := c.doJSON(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// BulkDeleteObjects deletes multiple objects in a single request (max 1000).
func (c *Client) BulkDeleteObjects(ctx context.Context, org, repo, sessionID string, paths []string) (*BulkDeleteResponse, error) {
	params := url.Values{}
	params.Set("session_id", sessionID)
	apiPath := fmt.Sprintf("/organizations/%s/repositories/%s/objects/delete?%s", org, repo, params.Encode())

	req := &BulkDeleteRequest{Paths: paths}
	var resp BulkDeleteResponse
	_, err := c.doJSON(ctx, http.MethodPost, apiPath, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListObjectsParams holds parameters for listing objects.
type ListObjectsParams struct {
	SessionID string
	Prefix    string
	After     string
	Amount    int
	Delimiter string
}

// ListObjects lists objects in a repository.
func (c *Client) ListObjects(ctx context.Context, org, repo string, params ListObjectsParams) (*ListObjectsResponse, error) {
	qp := url.Values{}
	if params.SessionID != "" {
		qp.Set("session_id", params.SessionID)
	}
	if params.Prefix != "" {
		qp.Set("prefix", params.Prefix)
	}
	if params.After != "" {
		qp.Set("after", params.After)
	}
	if params.Amount > 0 {
		qp.Set("amount", strconv.Itoa(params.Amount))
	}
	if params.Delimiter != "" {
		qp.Set("delimiter", params.Delimiter)
	}
	path := fmt.Sprintf("/organizations/%s/repositories/%s/objects?%s", org, repo, qp.Encode())

	var resp ListObjectsResponse
	_, err := c.doJSON(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

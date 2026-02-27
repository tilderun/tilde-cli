package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CreateSession starts a new session.
func (c *Client) CreateSession(ctx context.Context, org, repo string) (*CreateSessionResponse, error) {
	path := fmt.Sprintf("/organizations/%s/repositories/%s/sessions", org, repo)
	var resp CreateSessionResponse
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CommitSession commits a session. Returns the response which may indicate
// approval is required (HTTP 202).
func (c *Client) CommitSession(ctx context.Context, org, repo, sessionID string, req *CommitRequest) (*CommitResponse, error) {
	path := fmt.Sprintf("/organizations/%s/repositories/%s/sessions/%s", org, repo, sessionID)

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpResp, err := c.do(ctx, http.MethodPost, path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		return nil, parseAPIError(httpResp)
	}

	var resp CommitResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if httpResp.StatusCode == http.StatusAccepted {
		resp.ApprovalRequired = true
	}

	return &resp, nil
}

// RollbackSession deletes/rolls back a session.
func (c *Client) RollbackSession(ctx context.Context, org, repo, sessionID string) error {
	path := fmt.Sprintf("/organizations/%s/repositories/%s/sessions/%s", org, repo, sessionID)
	_, err := c.doJSON(ctx, http.MethodDelete, path, nil, nil)
	return err
}

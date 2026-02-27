package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ListRepositoriesParams holds parameters for listing repositories.
type ListRepositoriesParams struct {
	After  string
	Amount int
}

// ListRepositories lists repositories accessible to the caller.
// If org is non-empty, only repositories in that organization are returned.
func (c *Client) ListRepositories(ctx context.Context, org string, params ListRepositoriesParams) (*ListRepositoriesResponse, error) {
	qp := url.Values{}
	if params.After != "" {
		qp.Set("after", params.After)
	}
	if params.Amount > 0 {
		qp.Set("amount", strconv.Itoa(params.Amount))
	}

	var path string
	if org != "" {
		path = fmt.Sprintf("/organizations/%s/repositories?%s", org, qp.Encode())
	} else {
		path = fmt.Sprintf("/repositories?%s", qp.Encode())
	}

	var resp ListRepositoriesResponse
	_, err := c.doJSON(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

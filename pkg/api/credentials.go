package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// metadataRefreshLeeway is how long before expiry we proactively refresh
// sandbox-principal tokens. The server mints 15-minute tokens; refreshing
// at 2 minutes remaining gives plenty of headroom for clock skew and
// in-flight requests while still keeping the common path cached.
const metadataRefreshLeeway = 2 * time.Minute

// Credentials is the response shape returned by the sandbox identity
// metadata endpoint (see tilde specs/sandbox/identity_metadata_endpoint.md).
type Credentials struct {
	AccessToken    string    `json:"access_token"`
	ExpiresAt      time.Time `json:"expires_at"`
	PrincipalType  string    `json:"principal_type"`
	PrincipalID    string    `json:"principal_id"`
	PrincipalName  string    `json:"principal_name"`
	OrganizationID string    `json:"organization_id"`
	APIURL         string    `json:"api_url"`
}

// TokenSource returns a bearer token for API requests. Implementations
// may cache and refresh the token transparently.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// StaticTokenSource returns a fixed token forever. Used for long-lived
// API keys (tuk-, trk-, tak-).
type StaticTokenSource struct {
	Value string
}

func (s StaticTokenSource) Token(_ context.Context) (string, error) {
	return s.Value, nil
}

// MetadataTokenSource fetches short-lived sandbox-principal credentials
// from the in-pod metadata endpoint and refreshes them before expiry.
// Mirrors the AWS ECS/IMDS credentials-URI convention.
type MetadataTokenSource struct {
	URI    string
	Client *http.Client

	mu    sync.Mutex
	creds *Credentials
}

// NewMetadataTokenSource constructs a metadata token source. The URI is
// typically read from TILDE_SANDBOX_CREDENTIALS_URI and points at
// http://169.254.170.2/v1/credentials inside the sandbox pod.
func NewMetadataTokenSource(uri string) *MetadataTokenSource {
	return &MetadataTokenSource{
		URI: uri,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Fetch returns cached credentials if they are not near expiry, otherwise
// fetches a fresh set from the metadata endpoint.
func (m *MetadataTokenSource) Fetch(ctx context.Context) (*Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.creds != nil && time.Until(m.creds.ExpiresAt) > metadataRefreshLeeway {
		return m.creds, nil
	}

	creds, err := m.fetchLocked(ctx)
	if err != nil {
		return nil, err
	}
	m.creds = creds
	return m.creds, nil
}

func (m *MetadataTokenSource) fetchLocked(ctx context.Context) (*Credentials, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating metadata request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := m.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching sandbox credentials: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 256 {
			snippet = snippet[:256]
		}
		return nil, fmt.Errorf("sandbox metadata endpoint returned HTTP %d: %s", resp.StatusCode, snippet)
	}

	var creds Credentials
	if err := json.Unmarshal(body, &creds); err != nil {
		return nil, fmt.Errorf("decoding sandbox credentials: %w", err)
	}
	if creds.AccessToken == "" {
		return nil, fmt.Errorf("sandbox metadata response missing access_token")
	}
	if creds.APIURL == "" {
		return nil, fmt.Errorf("sandbox metadata response missing api_url")
	}
	return &creds, nil
}

// Token implements TokenSource.
func (m *MetadataTokenSource) Token(ctx context.Context) (string, error) {
	creds, err := m.Fetch(ctx)
	if err != nil {
		return "", err
	}
	return creds.AccessToken, nil
}

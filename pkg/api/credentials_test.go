package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestStaticTokenSource(t *testing.T) {
	s := StaticTokenSource{Value: "tuk-abc"}
	got, err := s.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got != "tuk-abc" {
		t.Errorf("Token = %q, want %q", got, "tuk-abc")
	}
}

func TestMetadataTokenSource_Fetch(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Credentials{
			AccessToken:    "tst-fresh",
			ExpiresAt:      time.Now().Add(15 * time.Minute),
			PrincipalType:  "agent",
			PrincipalID:    "p-123",
			PrincipalName:  "deploy-bot",
			OrganizationID: "org-1",
			APIURL:         "https://api.tilde.run",
		})
	}))
	defer srv.Close()

	src := NewMetadataTokenSource(srv.URL)
	creds, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if creds.AccessToken != "tst-fresh" {
		t.Errorf("AccessToken = %q", creds.AccessToken)
	}
	if creds.APIURL != "https://api.tilde.run" {
		t.Errorf("APIURL = %q", creds.APIURL)
	}
	if creds.PrincipalName != "deploy-bot" {
		t.Errorf("PrincipalName = %q", creds.PrincipalName)
	}

	// Second Fetch should be cached — expiry is far in the future.
	if _, err := src.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("upstream called %d times, want 1 (cached)", got)
	}
}

func TestMetadataTokenSource_RefreshesBeforeExpiry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		// First call: near expiry. Second: fresh.
		expiry := time.Now().Add(15 * time.Minute)
		token := "tst-fresh"
		if n == 1 {
			expiry = time.Now().Add(30 * time.Second) // inside leeway
			token = "tst-stale"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Credentials{
			AccessToken: token,
			ExpiresAt:   expiry,
			APIURL:      "https://api.tilde.run",
		})
	}))
	defer srv.Close()

	src := NewMetadataTokenSource(srv.URL)
	tok1, err := src.Token(context.Background())
	if err != nil {
		t.Fatalf("Token #1: %v", err)
	}
	if tok1 != "tst-stale" {
		t.Errorf("first token = %q, want tst-stale", tok1)
	}

	// Second call should refresh because the cached token is inside the leeway.
	tok2, err := src.Token(context.Background())
	if err != nil {
		t.Fatalf("Token #2: %v", err)
	}
	if tok2 != "tst-fresh" {
		t.Errorf("second token = %q, want tst-fresh", tok2)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("upstream called %d times, want 2 (refresh)", got)
	}
}

func TestMetadataTokenSource_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message":"sandbox terminal"}`))
	}))
	defer srv.Close()

	src := NewMetadataTokenSource(srv.URL)
	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error from 409 response")
	}
}

func TestMetadataTokenSource_MissingAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Credentials{
			ExpiresAt: time.Now().Add(15 * time.Minute),
			APIURL:    "https://api.tilde.run",
		})
	}))
	defer srv.Close()

	src := NewMetadataTokenSource(srv.URL)
	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for missing access_token")
	}
}

func TestMetadataTokenSource_MissingAPIURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Credentials{
			AccessToken: "tst-xxx",
			ExpiresAt:   time.Now().Add(15 * time.Minute),
		})
	}))
	defer srv.Close()

	src := NewMetadataTokenSource(srv.URL)
	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for missing api_url")
	}
}

func TestClient_UsesTokenSource(t *testing.T) {
	// Verify that Client re-reads the token from its source per request, so
	// a rotating source is reflected in outgoing Authorization headers.
	src := &rotatingTokenSource{tokens: []string{"tst-one", "tst-two"}}

	var gotAuths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuths = append(gotAuths, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClientWithTokenSource(srv.URL, src)
	for i := 0; i < 2; i++ {
		resp, err := c.do(context.Background(), http.MethodGet, "/x", nil)
		if err != nil {
			t.Fatalf("do #%d: %v", i, err)
		}
		resp.Body.Close()
	}

	want := []string{"Bearer tst-one", "Bearer tst-two"}
	for i, w := range want {
		if gotAuths[i] != w {
			t.Errorf("request %d auth = %q, want %q", i, gotAuths[i], w)
		}
	}
}

type rotatingTokenSource struct {
	tokens []string
	i      int
}

func (r *rotatingTokenSource) Token(_ context.Context) (string, error) {
	t := r.tokens[r.i%len(r.tokens)]
	r.i++
	return t, nil
}

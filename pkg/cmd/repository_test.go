package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tilderun/tilde-cli/pkg/api"
)

func setupTestEnv(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("TILDE_API_KEY", "tuk-testkey")
	t.Setenv("TILDE_ENDPOINT_URL", srv.URL)
}

func TestRepositoryLs_All(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/repositories" {
			t.Errorf("path = %s, want /api/v1/repositories", r.URL.Path)
		}
		json.NewEncoder(w).Encode(api.ListRepositoriesResponse{
			Results: []api.Repository{
				{OrganizationSlug: "acme", Name: "data"},
				{OrganizationSlug: "acme", Name: "models"},
			},
			Pagination: api.Pagination{HasMore: false},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestRepositoryLs_ByOrg(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/organizations/myorg/repositories" {
			t.Errorf("path = %s, want /api/v1/organizations/myorg/repositories", r.URL.Path)
		}
		json.NewEncoder(w).Encode(api.ListRepositoriesResponse{
			Results: []api.Repository{
				{OrganizationSlug: "myorg", Name: "repo1"},
			},
			Pagination: api.Pagination{HasMore: false},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls", "myorg"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestRepositoryLs_Pagination(t *testing.T) {
	callCount := 0
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			json.NewEncoder(w).Encode(api.ListRepositoriesResponse{
				Results:    []api.Repository{{OrganizationSlug: "org", Name: "repo1"}},
				Pagination: api.Pagination{HasMore: true, NextOffset: "repo1"},
			})
		} else {
			if r.URL.Query().Get("after") != "repo1" {
				t.Errorf("after = %q, want %q", r.URL.Query().Get("after"), "repo1")
			}
			json.NewEncoder(w).Encode(api.ListRepositoriesResponse{
				Results:    []api.Repository{{OrganizationSlug: "org", Name: "repo2"}},
				Pagination: api.Pagination{HasMore: false},
			})
		}
	})

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

func TestRepositoryLs_APIError(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for API error")
	}
}

func TestRepositoryLs_Empty(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.ListRepositoriesResponse{
			Results:    []api.Repository{},
			Pagination: api.Pagination{HasMore: false},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"repository", "ls", "emptyorg"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

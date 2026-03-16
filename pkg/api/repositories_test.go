package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListRepositories_All(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/repositories" {
			t.Errorf("path = %s, want /repositories", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ListRepositoriesResponse{
			Results: []Repository{{OrganizationSlug: "org1", Name: "repo1"}},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	resp, err := c.ListRepositories(context.Background(), "", ListRepositoriesParams{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if resp.Results[0].Name != "repo1" {
		t.Errorf("Name = %q, want %q", resp.Results[0].Name, "repo1")
	}
}

func TestListRepositories_WithOrg(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations/myorg/repositories" {
			t.Errorf("path = %s, want /organizations/myorg/repositories", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ListRepositoriesResponse{
			Results: []Repository{{OrganizationSlug: "myorg", Name: "r1"}},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	resp, err := c.ListRepositories(context.Background(), "myorg", ListRepositoriesParams{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].OrganizationSlug != "myorg" {
		t.Errorf("unexpected results: %+v", resp.Results)
	}
}

func TestListRepositories_OrgWithSpecialChars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// After decoding, the path should contain the org name in the correct segment
		want := "/organizations/my org/repositories"
		if r.URL.Path != want {
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		json.NewEncoder(w).Encode(ListRepositoriesResponse{})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	_, err := c.ListRepositories(context.Background(), "my org", ListRepositoriesParams{})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
}

func TestListRepositories_Pagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		amount := r.URL.Query().Get("amount")
		after := r.URL.Query().Get("after")
		if amount != "10" {
			t.Errorf("amount = %q, want %q", amount, "10")
		}
		if after != "cursor-abc" {
			t.Errorf("after = %q, want %q", after, "cursor-abc")
		}
		json.NewEncoder(w).Encode(ListRepositoriesResponse{
			Results:    []Repository{{OrganizationSlug: "org", Name: "r2"}},
			Pagination: Pagination{HasMore: false},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	resp, err := c.ListRepositories(context.Background(), "", ListRepositoriesParams{
		After:  "cursor-abc",
		Amount: 10,
	})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if resp.Pagination.HasMore {
		t.Error("expected HasMore=false")
	}
}

func TestListRepositories_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tuk-key")
	_, err := c.ListRepositories(context.Background(), "org", ListRepositoriesParams{})
	if err == nil {
		t.Fatal("expected error")
	}
}

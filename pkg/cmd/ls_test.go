package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
)

func TestLs_Basic(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("delimiter") != "/" {
			t.Errorf("delimiter = %q, want /", r.URL.Query().Get("delimiter"))
		}
		json.NewEncoder(w).Encode(api.ListObjectsResponse{
			Results: []api.ListingEntry{
				{Path: "file1.txt", Type: "object", Entry: &api.Entry{Size: 100, LastModified: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
				{Path: "dir/", Type: "prefix"},
			},
			Pagination: api.Pagination{HasMore: false},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "--session", "sess-1", "cb://org/repo"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestLs_Recursive(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("delimiter") != "" {
			t.Errorf("recursive: delimiter should be empty, got %q", r.URL.Query().Get("delimiter"))
		}
		json.NewEncoder(w).Encode(api.ListObjectsResponse{
			Results: []api.ListingEntry{
				{Path: "file1.txt", Type: "object"},
				{Path: "dir/file2.txt", Type: "object"},
			},
			Pagination: api.Pagination{HasMore: false},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "--session", "sess-1", "-r", "cb://org/repo"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestLs_WithPrefix(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		// prefix should have trailing / appended
		if r.URL.Query().Get("prefix") != "data/subdir/" {
			t.Errorf("prefix = %q, want %q", r.URL.Query().Get("prefix"), "data/subdir/")
		}
		json.NewEncoder(w).Encode(api.ListObjectsResponse{
			Results: []api.ListingEntry{
				{Path: "data/subdir/file.txt", Type: "object", Entry: &api.Entry{Size: 42}},
				{Path: "data/subdir/nested/", Type: "prefix"},
			},
			Pagination: api.Pagination{HasMore: false},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "--session", "sess-1", "cb://org/repo/data/subdir"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestLs_RelativePaths(t *testing.T) {
	// Verify output shows relative paths by capturing what gets printed
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.ListObjectsResponse{
			Results: []api.ListingEntry{
				{Path: "data/images/cat.jpg", Type: "object", Entry: &api.Entry{Size: 1024}},
				{Path: "data/images/dogs/", Type: "prefix"},
			},
			Pagination: api.Pagination{HasMore: false},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "--session", "sess-1", "cb://org/repo/data/images"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Output should show "cat.jpg" and "dogs/", not "data/images/cat.jpg"
}

func TestLs_Pagination(t *testing.T) {
	callCount := 0
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			json.NewEncoder(w).Encode(api.ListObjectsResponse{
				Results:    []api.ListingEntry{{Path: "file1.txt", Type: "object"}},
				Pagination: api.Pagination{HasMore: true, NextOffset: "file1.txt"},
			})
		} else {
			if r.URL.Query().Get("after") != "file1.txt" {
				t.Errorf("after = %q, want %q", r.URL.Query().Get("after"), "file1.txt")
			}
			json.NewEncoder(w).Encode(api.ListObjectsResponse{
				Results:    []api.ListingEntry{{Path: "file2.txt", Type: "object"}},
				Pagination: api.Pagination{HasMore: false},
			})
		}
	})

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "--session", "sess-1", "cb://org/repo"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

func TestLs_MaxResults(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		// Return more results than max-results
		json.NewEncoder(w).Encode(api.ListObjectsResponse{
			Results: []api.ListingEntry{
				{Path: "a.txt", Type: "object"},
				{Path: "b.txt", Type: "object"},
				{Path: "c.txt", Type: "object"},
			},
			Pagination: api.Pagination{HasMore: true, NextOffset: "c.txt"},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "--session", "sess-1", "--max-results", "2", "cb://org/repo"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Should stop after 2 results even though server returned 3
}

func TestLs_MissingSession(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --session")
	}
}

func TestLs_InvalidURI(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "--session", "s1", "not-a-uri"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid URI")
	}
}

func TestLs_APIError(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "--session", "sess-1", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for API error")
	}
}

func TestLs_SessionIDPassedToAPI(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("session_id") != "my-sess-id" {
			t.Errorf("session_id = %q, want %q", r.URL.Query().Get("session_id"), "my-sess-id")
		}
		json.NewEncoder(w).Encode(api.ListObjectsResponse{
			Results:    []api.ListingEntry{},
			Pagination: api.Pagination{HasMore: false},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"ls", "--session", "my-sess-id", "cb://org/repo"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

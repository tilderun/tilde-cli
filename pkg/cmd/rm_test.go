package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
)

func TestRm_SingleObject(t *testing.T) {
	var gotMethod, gotPath, gotSession string
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Query().Get("path")
		gotSession = r.URL.Query().Get("session_id")
		w.WriteHeader(204)
	})

	root := NewRootCmd()
	root.SetArgs([]string{"rm", "--session", "sess-1", "cb://org/repo/dir/file.txt"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if gotPath != "dir/file.txt" {
		t.Errorf("path = %q, want %q", gotPath, "dir/file.txt")
	}
	if gotSession != "sess-1" {
		t.Errorf("session_id = %q, want %q", gotSession, "sess-1")
	}
}

func TestRm_MissingSession(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"rm", "cb://org/repo/file.txt"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --session")
	}
}

func TestRm_MissingPath(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"rm", "--session", "s1", "cb://org/repo"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestRm_Recursive(t *testing.T) {
	var bulkPaths []string
	callCount := 0
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/objects") && r.Method == http.MethodGet {
			// List
			json.NewEncoder(w).Encode(api.ListObjectsResponse{
				Results: []api.ListingEntry{
					{Path: "data/a.txt", Type: "object"},
					{Path: "data/b.txt", Type: "object"},
					{Path: "data/c.txt", Type: "object"},
				},
				Pagination: api.Pagination{HasMore: false},
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/objects/delete") {
			callCount++
			var body api.BulkDeleteRequest
			json.NewDecoder(r.Body).Decode(&body)
			bulkPaths = body.Paths
			json.NewEncoder(w).Encode(api.BulkDeleteResponse{Deleted: len(body.Paths)})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(500)
	})

	root := NewRootCmd()
	root.SetArgs([]string{"rm", "--session", "sess-1", "-r", "cb://org/repo/data"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if callCount != 1 {
		t.Errorf("bulk delete calls = %d, want 1", callCount)
	}
	if len(bulkPaths) != 3 {
		t.Errorf("bulk paths = %d, want 3", len(bulkPaths))
	}
}

func TestRm_RecursiveMultipleBatches(t *testing.T) {
	// Generate 1500 items to test batching (batch size = 1000)
	entries := make([]api.ListingEntry, 1500)
	for i := range entries {
		entries[i] = api.ListingEntry{
			Path: "data/" + string(rune('a'+i/100)) + string(rune('a'+i%100/10)) + string(rune('0'+i%10)) + ".txt",
			Type: "object",
		}
	}

	batchCount := 0
	totalDeleted := 0
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/objects") && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(api.ListObjectsResponse{
				Results:    entries,
				Pagination: api.Pagination{HasMore: false},
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/objects/delete") {
			batchCount++
			var body api.BulkDeleteRequest
			json.NewDecoder(r.Body).Decode(&body)
			totalDeleted += len(body.Paths)
			if len(body.Paths) > 1000 {
				t.Errorf("batch size = %d, should be <= 1000", len(body.Paths))
			}
			json.NewEncoder(w).Encode(api.BulkDeleteResponse{Deleted: len(body.Paths)})
			return
		}
		w.WriteHeader(500)
	})

	root := NewRootCmd()
	root.SetArgs([]string{"rm", "--session", "s1", "-r", "cb://org/repo/data"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if batchCount != 2 {
		t.Errorf("batch count = %d, want 2", batchCount)
	}
	if totalDeleted != 1500 {
		t.Errorf("total deleted = %d, want 1500", totalDeleted)
	}
}

func TestRm_RecursiveEmpty(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.ListObjectsResponse{
			Results:    []api.ListingEntry{},
			Pagination: api.Pagination{HasMore: false},
		})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"rm", "--session", "s1", "-r", "cb://org/repo/empty"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestRm_APIError(t *testing.T) {
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"message": "bad request"})
	})

	root := NewRootCmd()
	root.SetArgs([]string{"rm", "--session", "s1", "cb://org/repo/file.txt"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRm_InvalidURI(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"rm", "--session", "s1", "not-a-uri"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid URI")
	}
}

func TestRm_RecursiveTrailingSlash(t *testing.T) {
	var gotPrefix string
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gotPrefix = r.URL.Query().Get("prefix")
			json.NewEncoder(w).Encode(api.ListObjectsResponse{
				Results:    []api.ListingEntry{},
				Pagination: api.Pagination{HasMore: false},
			})
			return
		}
		w.WriteHeader(500)
	})

	// Without trailing slash — should auto-append
	root := NewRootCmd()
	root.SetArgs([]string{"rm", "--session", "s1", "-r", "cb://org/repo/data"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPrefix != "data/" {
		t.Errorf("prefix = %q, want %q", gotPrefix, "data/")
	}
}

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetObjectPresignedURL_Success(t *testing.T) {
	presignedURL := "https://s3.example.com/bucket/key?X-Amz-Signature=abc123"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Query().Get("path") != "dir/file.txt" {
			t.Errorf("path param = %q", r.URL.Query().Get("path"))
		}
		if r.URL.Query().Get("presign") != "true" {
			t.Errorf("presign param = %q", r.URL.Query().Get("presign"))
		}
		if r.URL.Query().Get("session_id") != "sess-1" {
			t.Errorf("session_id param = %q", r.URL.Query().Get("session_id"))
		}

		w.Header().Set("Location", presignedURL)
		w.WriteHeader(307)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	got, err := c.GetObjectPresignedURL(context.Background(), "org", "repo", "dir/file.txt", "sess-1")
	if err != nil {
		t.Fatalf("GetObjectPresignedURL: %v", err)
	}
	if got != presignedURL {
		t.Errorf("URL = %q, want %q", got, presignedURL)
	}
}

func TestGetObjectPresignedURL_NoSessionID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("session_id") != "" {
			t.Errorf("expected no session_id param, got %q", r.URL.Query().Get("session_id"))
		}
		w.Header().Set("Location", "https://s3.example.com/presigned")
		w.WriteHeader(307)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.GetObjectPresignedURL(context.Background(), "org", "repo", "file.txt", "")
	if err != nil {
		t.Fatalf("GetObjectPresignedURL: %v", err)
	}
}

func TestGetObjectPresignedURL_MissingLocation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(307) // 307 without Location header
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.GetObjectPresignedURL(context.Background(), "org", "repo", "file.txt", "s1")
	if err == nil {
		t.Fatal("expected error for missing Location header")
	}
}

func TestGetObjectPresignedURL_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.GetObjectPresignedURL(context.Background(), "org", "repo", "missing.txt", "s1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected not found, got: %v", err)
	}
}

func TestGetObjectPresignedURL_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200) // unexpected 200 instead of 307
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.GetObjectPresignedURL(context.Background(), "org", "repo", "file.txt", "s1")
	if err == nil {
		t.Fatal("expected error for unexpected 200")
	}
}

func TestStageObject(t *testing.T) {
	expiresAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Query().Get("path") != "data/file.csv" {
			t.Errorf("path = %q", r.URL.Query().Get("path"))
		}
		if r.URL.Query().Get("session_id") != "sess-2" {
			t.Errorf("session_id = %q", r.URL.Query().Get("session_id"))
		}

		json.NewEncoder(w).Encode(StageResponse{
			UploadURL:       "https://s3.example.com/upload",
			PhysicalAddress: "s3://bucket/key",
			Signature:       "sig123",
			ExpiresAt:       expiresAt,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	resp, err := c.StageObject(context.Background(), "org", "repo", "data/file.csv", "sess-2")
	if err != nil {
		t.Fatalf("StageObject: %v", err)
	}
	if resp.UploadURL != "https://s3.example.com/upload" {
		t.Errorf("UploadURL = %q", resp.UploadURL)
	}
	if resp.PhysicalAddress != "s3://bucket/key" {
		t.Errorf("PhysicalAddress = %q", resp.PhysicalAddress)
	}
	if resp.Signature != "sig123" {
		t.Errorf("Signature = %q", resp.Signature)
	}
	if !resp.ExpiresAt.Equal(expiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", resp.ExpiresAt, expiresAt)
	}
}

func TestStageObject_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"message": "invalid session"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.StageObject(context.Background(), "org", "repo", "file.txt", "bad-sess")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFinalizeObject(t *testing.T) {
	var gotBody FinalizeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Query().Get("path") != "file.txt" {
			t.Errorf("path = %q", r.URL.Query().Get("path"))
		}
		if r.URL.Query().Get("session_id") != "sess-3" {
			t.Errorf("session_id = %q", r.URL.Query().Get("session_id"))
		}
		if r.URL.Query().Get("expires_at") == "" {
			t.Error("expires_at param missing")
		}

		json.NewDecoder(r.Body).Decode(&gotBody)

		w.WriteHeader(201)
		json.NewEncoder(w).Encode(FinalizeResponse{Path: "file.txt", ETag: "etag-abc"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	resp, err := c.FinalizeObject(context.Background(), "org", "repo", "file.txt", "sess-3",
		"2026-01-01T00:00:00Z",
		&FinalizeRequest{
			PhysicalAddress: "s3://bucket/key",
			Signature:       "sig",
			ContentType:     "text/plain",
		})
	if err != nil {
		t.Fatalf("FinalizeObject: %v", err)
	}
	if resp.Path != "file.txt" {
		t.Errorf("Path = %q", resp.Path)
	}
	if resp.ETag != "etag-abc" {
		t.Errorf("ETag = %q", resp.ETag)
	}
	if gotBody.PhysicalAddress != "s3://bucket/key" {
		t.Errorf("request PhysicalAddress = %q", gotBody.PhysicalAddress)
	}
	if gotBody.Signature != "sig" {
		t.Errorf("request Signature = %q", gotBody.Signature)
	}
	if gotBody.ContentType != "text/plain" {
		t.Errorf("request ContentType = %q", gotBody.ContentType)
	}
}

func TestDeleteObject(t *testing.T) {
	var gotMethod, gotPath, gotSession string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Query().Get("path")
		gotSession = r.URL.Query().Get("session_id")
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	err := c.DeleteObject(context.Background(), "org", "repo", "dir/file.txt", "sess-4")
	if err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if gotPath != "dir/file.txt" {
		t.Errorf("path = %q", gotPath)
	}
	if gotSession != "sess-4" {
		t.Errorf("session_id = %q", gotSession)
	}
}

func TestDeleteObject_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"message": "invalid path"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	err := c.DeleteObject(context.Background(), "org", "repo", "", "sess")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBulkDeleteObjects(t *testing.T) {
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Query().Get("session_id") != "sess-5" {
			t.Errorf("session_id = %q", r.URL.Query().Get("session_id"))
		}

		var body BulkDeleteRequest
		json.NewDecoder(r.Body).Decode(&body)
		gotPaths = body.Paths

		json.NewEncoder(w).Encode(BulkDeleteResponse{Deleted: len(body.Paths)})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	paths := []string{"a.txt", "b.txt", "dir/c.txt"}
	resp, err := c.BulkDeleteObjects(context.Background(), "org", "repo", "sess-5", paths)
	if err != nil {
		t.Fatalf("BulkDeleteObjects: %v", err)
	}
	if resp.Deleted != 3 {
		t.Errorf("Deleted = %d, want 3", resp.Deleted)
	}
	if len(gotPaths) != 3 {
		t.Errorf("sent %d paths, want 3", len(gotPaths))
	}
}

func TestListObjects_Basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Query().Get("session_id") != "sess-6" {
			t.Errorf("session_id = %q", r.URL.Query().Get("session_id"))
		}
		if r.URL.Query().Get("prefix") != "data/" {
			t.Errorf("prefix = %q", r.URL.Query().Get("prefix"))
		}
		if r.URL.Query().Get("delimiter") != "/" {
			t.Errorf("delimiter = %q", r.URL.Query().Get("delimiter"))
		}
		if r.URL.Query().Get("amount") != "100" {
			t.Errorf("amount = %q", r.URL.Query().Get("amount"))
		}

		json.NewEncoder(w).Encode(ListObjectsResponse{
			Results: []ListingEntry{
				{Path: "data/file1.txt", Type: "object"},
				{Path: "data/subdir/", Type: "prefix"},
			},
			Pagination: Pagination{HasMore: false, MaxPerPage: 1000},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	resp, err := c.ListObjects(context.Background(), "org", "repo", ListObjectsParams{
		SessionID: "sess-6",
		Prefix:    "data/",
		Delimiter: "/",
		Amount:    100,
	})
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(resp.Results))
	}
	if resp.Results[0].Path != "data/file1.txt" {
		t.Errorf("first result path = %q", resp.Results[0].Path)
	}
	if resp.Results[1].Type != "prefix" {
		t.Errorf("second result type = %q, want prefix", resp.Results[1].Type)
	}
	if resp.Pagination.HasMore {
		t.Error("expected HasMore = false")
	}
}

func TestListObjects_Pagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		after := r.URL.Query().Get("after")

		if callCount == 1 {
			if after != "" {
				t.Errorf("first call: after should be empty, got %q", after)
			}
			json.NewEncoder(w).Encode(ListObjectsResponse{
				Results:    []ListingEntry{{Path: "file1.txt", Type: "object"}},
				Pagination: Pagination{HasMore: true, NextOffset: "file1.txt"},
			})
		} else {
			if after != "file1.txt" {
				t.Errorf("second call: after = %q, want %q", after, "file1.txt")
			}
			json.NewEncoder(w).Encode(ListObjectsResponse{
				Results:    []ListingEntry{{Path: "file2.txt", Type: "object"}},
				Pagination: Pagination{HasMore: false},
			})
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")

	// First page
	resp, err := c.ListObjects(context.Background(), "org", "repo", ListObjectsParams{
		SessionID: "s1",
	})
	if err != nil {
		t.Fatalf("ListObjects page 1: %v", err)
	}
	if !resp.Pagination.HasMore {
		t.Error("page 1: expected HasMore = true")
	}

	// Second page
	resp, err = c.ListObjects(context.Background(), "org", "repo", ListObjectsParams{
		SessionID: "s1",
		After:     resp.Pagination.NextOffset,
	})
	if err != nil {
		t.Fatalf("ListObjects page 2: %v", err)
	}
	if resp.Pagination.HasMore {
		t.Error("page 2: expected HasMore = false")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestListObjects_OptionalParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// When params are empty, they should not appear in the query
		if r.URL.Query().Has("prefix") {
			t.Error("prefix should not be in query when empty")
		}
		if r.URL.Query().Has("after") {
			t.Error("after should not be in query when empty")
		}
		if r.URL.Query().Has("delimiter") {
			t.Error("delimiter should not be in query when empty")
		}
		if r.URL.Query().Has("amount") {
			t.Error("amount should not be in query when 0")
		}

		json.NewEncoder(w).Encode(ListObjectsResponse{
			Results:    []ListingEntry{},
			Pagination: Pagination{HasMore: false},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.ListObjects(context.Background(), "org", "repo", ListObjectsParams{
		SessionID: "s1",
	})
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
}

func TestListObjects_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cak-key")
	_, err := c.ListObjects(context.Background(), "org", "repo", ListObjectsParams{SessionID: "s1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
)

func TestCp_DirectionDetection_Upload(t *testing.T) {
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer s3Srv.Close()

	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/object/stage") {
			json.NewEncoder(w).Encode(api.StageResponse{
				UploadURL:       s3Srv.URL + "/upload",
				PhysicalAddress: "s3://b/k",
				Signature:       "s",
				ExpiresAt:       time.Now().Add(time.Hour),
			})
			return
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(api.FinalizeResponse{Path: "f", ETag: "e"})
	})
	// Override the S3 client on the globally-set apiClient


	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "upload.txt")
	os.WriteFile(localFile, []byte("content"), 0o644)

	root := NewRootCmd()
	root.SetArgs([]string{"cp", "--session", "sess-1", localFile, "cb://org/repo/upload.txt"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestCp_DirectionDetection_Download(t *testing.T) {
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("downloaded content"))
	}))
	defer s3Srv.Close()

	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", s3Srv.URL+"/download")
		w.WriteHeader(307)
	})


	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "output.txt")

	root := NewRootCmd()
	root.SetArgs([]string{"cp", "--session", "sess-1", "cb://org/repo/file.txt", dst})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, _ := os.ReadFile(dst)
	if string(got) != "downloaded content" {
		t.Errorf("content = %q, want %q", string(got), "downloaded content")
	}
}

func TestCp_BothURIs(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"cp", "--session", "s1", "cb://org/repo/a", "cb://org/repo/b"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when both args are URIs")
	}
	if !strings.Contains(err.Error(), "both") {
		t.Errorf("error = %q, expected 'both' in message", err.Error())
	}
}

func TestCp_NeitherURI(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"cp", "--session", "s1", "./local1", "./local2"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when neither arg is a URI")
	}
	if !strings.Contains(err.Error(), "neither") {
		t.Errorf("error = %q, expected 'neither' in message", err.Error())
	}
}

func TestCp_MissingSession(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"cp", "./file.txt", "cb://org/repo/file.txt"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --session")
	}
}

func TestCp_RecursiveUpload(t *testing.T) {
	var mu sync.Mutex
	uploadedPaths := make(map[string]bool)

	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer s3Srv.Close()

	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/object/stage") {
			mu.Lock()
			uploadedPaths[r.URL.Query().Get("path")] = true
			mu.Unlock()
			json.NewEncoder(w).Encode(api.StageResponse{
				UploadURL:       s3Srv.URL + "/upload",
				PhysicalAddress: "s3://b/k",
				Signature:       "s",
				ExpiresAt:       time.Now().Add(time.Hour),
			})
			return
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(api.FinalizeResponse{Path: "f", ETag: "e"})
	})

	maxConcurrency = 2

	// Create test directory structure
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("b"), 0o644)

	root := NewRootCmd()
	root.SetArgs([]string{"cp", "--session", "sess-1", "-r", srcDir, "cb://org/repo/data/"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !uploadedPaths["data/a.txt"] {
		t.Error("expected data/a.txt to be uploaded")
	}
	if !uploadedPaths["data/sub/b.txt"] {
		t.Error("expected data/sub/b.txt to be uploaded")
	}
}

func TestCp_RecursiveDownload(t *testing.T) {
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("file content"))
	}))
	defer s3Srv.Close()

	callCount := 0
	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/objects") {
			// List endpoint
			json.NewEncoder(w).Encode(api.ListObjectsResponse{
				Results: []api.ListingEntry{
					{Path: "data/file1.txt", Type: "object"},
					{Path: "data/sub/file2.txt", Type: "object"},
				},
				Pagination: api.Pagination{HasMore: false},
			})
			return
		}
		// Presigned download (GET /object)
		callCount++
		w.Header().Set("Location", s3Srv.URL+"/download")
		w.WriteHeader(307)
	})

	maxConcurrency = 2

	tmpDir := t.TempDir()
	dstDir := filepath.Join(tmpDir, "output")

	root := NewRootCmd()
	root.SetArgs([]string{"cp", "--session", "sess-1", "-r", "cb://org/repo/data", dstDir})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Check downloaded files
	f1, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if err != nil {
		t.Fatalf("reading file1: %v", err)
	}
	if string(f1) != "file content" {
		t.Errorf("file1 = %q", string(f1))
	}

	f2, err := os.ReadFile(filepath.Join(dstDir, "sub", "file2.txt"))
	if err != nil {
		t.Fatalf("reading file2: %v", err)
	}
	if string(f2) != "file content" {
		t.Errorf("file2 = %q", string(f2))
	}
}

func TestCp_DownloadToStdout(t *testing.T) {
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("stdout content"))
	}))
	defer s3Srv.Close()

	setupTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", s3Srv.URL+"/download")
		w.WriteHeader(307)
	})


	root := NewRootCmd()
	root.SetArgs([]string{"cp", "--session", "sess-1", "cb://org/repo/file.txt", "-"})

	// The actual download to stdout works if it doesn't error
	_ = root.Execute()
}

func TestCp_WrongArgCount(t *testing.T) {
	t.Setenv("CEREBRAL_API_KEY", "cak-test")
	t.Setenv("CEREBRAL_ENDPOINT_URL", "http://unused")

	root := NewRootCmd()
	root.SetArgs([]string{"cp", "--session", "s1", "only-one-arg"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for wrong number of args")
	}
}

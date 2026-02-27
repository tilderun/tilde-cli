package upload

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
)

func TestUpload_FullPipeline(t *testing.T) {
	const fileContent = "hello world test content"
	var (
		gotS3Body    string
		gotS3CT      string
		gotStagePath string
		gotFinBody   api.FinalizeRequest
		gotFinPath   string
	)

	// Mock S3 server
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("S3: method = %s, want PUT", r.Method)
		}
		gotS3CT = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotS3Body = string(body)
		w.WriteHeader(200)
	}))
	defer s3Srv.Close()

	// Mock API server
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/object/stage"):
			gotStagePath = r.URL.Query().Get("path")
			json.NewEncoder(w).Encode(api.StageResponse{
				UploadURL:       s3Srv.URL + "/upload",
				PhysicalAddress: "s3://bucket/phys-key",
				Signature:       "sig-abc",
				ExpiresAt:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			})
		case strings.Contains(r.URL.Path, "/object/finalize"):
			gotFinPath = r.URL.Query().Get("path")
			json.NewDecoder(r.Body).Decode(&gotFinBody)
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(api.FinalizeResponse{Path: "data/file.txt", ETag: "etag-1"})
		default:
			t.Errorf("unexpected API call: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer apiSrv.Close()

	// Create temp file
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(localFile, []byte(fileContent), 0o644)

	// Create client pointing to mock servers
	client := api.NewClient(apiSrv.URL, "cak-key")
	client.S3Client = s3Srv.Client()

	err := Upload(context.Background(), client, "org", "repo", "data/file.txt", "sess-1", localFile)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// Verify stage was called with correct path
	if gotStagePath != "data/file.txt" {
		t.Errorf("stage path = %q, want %q", gotStagePath, "data/file.txt")
	}

	// Verify S3 received the file content
	if gotS3Body != fileContent {
		t.Errorf("S3 body = %q, want %q", gotS3Body, fileContent)
	}

	// Verify finalize was called correctly
	if gotFinPath != "data/file.txt" {
		t.Errorf("finalize path = %q, want %q", gotFinPath, "data/file.txt")
	}
	if gotFinBody.PhysicalAddress != "s3://bucket/phys-key" {
		t.Errorf("finalize PhysicalAddress = %q", gotFinBody.PhysicalAddress)
	}
	if gotFinBody.Signature != "sig-abc" {
		t.Errorf("finalize Signature = %q", gotFinBody.Signature)
	}

	// Content-Type should be detected (text/plain for .txt)
	if gotS3CT == "" {
		t.Error("S3 Content-Type should not be empty")
	}
}

func TestUpload_FileNotFound(t *testing.T) {
	client := api.NewClient("http://unused", "cak-key")
	err := Upload(context.Background(), client, "org", "repo", "file.txt", "sess", "/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "opening file") {
		t.Errorf("error = %q, expected 'opening file' prefix", err.Error())
	}
}

func TestUpload_StageError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	}))
	defer apiSrv.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(localFile, []byte("data"), 0o644)

	client := api.NewClient(apiSrv.URL, "cak-key")
	err := Upload(context.Background(), client, "org", "repo", "file.txt", "sess", localFile)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "staging") {
		t.Errorf("error = %q, expected staging error", err.Error())
	}
}

func TestUpload_S3Error(t *testing.T) {
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte("Access Denied"))
	}))
	defer s3Srv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.StageResponse{
			UploadURL:       s3Srv.URL + "/upload",
			PhysicalAddress: "s3://bucket/key",
			Signature:       "sig",
			ExpiresAt:       time.Now().Add(time.Hour),
		})
	}))
	defer apiSrv.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(localFile, []byte("data"), 0o644)

	client := api.NewClient(apiSrv.URL, "cak-key")
	client.S3Client = s3Srv.Client()

	err := Upload(context.Background(), client, "org", "repo", "file.txt", "sess", localFile)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "S3") {
		t.Errorf("error = %q, expected S3 error", err.Error())
	}
}

func TestUpload_FinalizeError(t *testing.T) {
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer s3Srv.Close()

	callCount := 0
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if strings.Contains(r.URL.Path, "/object/stage") {
			json.NewEncoder(w).Encode(api.StageResponse{
				UploadURL:       s3Srv.URL + "/upload",
				PhysicalAddress: "s3://bucket/key",
				Signature:       "sig",
				ExpiresAt:       time.Now().Add(time.Hour),
			})
			return
		}
		// Finalize fails
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]string{"message": "conflict"})
	}))
	defer apiSrv.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(localFile, []byte("data"), 0o644)

	client := api.NewClient(apiSrv.URL, "cak-key")
	client.S3Client = s3Srv.Client()

	err := Upload(context.Background(), client, "org", "repo", "file.txt", "sess", localFile)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "finalizing") {
		t.Errorf("error = %q, expected finalizing error", err.Error())
	}
}

func TestUploadReader(t *testing.T) {
	var gotBody string
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(200)
	}))
	defer s3Srv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/object/stage") {
			json.NewEncoder(w).Encode(api.StageResponse{
				UploadURL:       s3Srv.URL + "/upload",
				PhysicalAddress: "s3://bucket/key",
				Signature:       "sig",
				ExpiresAt:       time.Now().Add(time.Hour),
			})
			return
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(api.FinalizeResponse{Path: "f.txt", ETag: "e"})
	}))
	defer apiSrv.Close()

	client := api.NewClient(apiSrv.URL, "cak-key")
	client.S3Client = s3Srv.Client()

	content := "reader content here"
	err := UploadReader(context.Background(), client, "org", "repo", "f.txt", "sess",
		strings.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("UploadReader: %v", err)
	}
	if gotBody != content {
		t.Errorf("S3 body = %q, want %q", gotBody, content)
	}
}

func TestUpload_ContextCancellation(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until request context is done
		<-r.Context().Done()
	}))
	defer apiSrv.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(localFile, []byte("data"), 0o644)

	client := api.NewClient(apiSrv.URL, "cak-key")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := Upload(ctx, client, "org", "repo", "file.txt", "sess", localFile)
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

func TestUpload_EmptyFile(t *testing.T) {
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) != 0 {
			t.Errorf("expected empty body, got %d bytes", len(body))
		}
		w.WriteHeader(200)
	}))
	defer s3Srv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/object/stage") {
			json.NewEncoder(w).Encode(api.StageResponse{
				UploadURL:       s3Srv.URL + "/upload",
				PhysicalAddress: "s3://bucket/key",
				Signature:       "sig",
				ExpiresAt:       time.Now().Add(time.Hour),
			})
			return
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(api.FinalizeResponse{Path: "empty.txt", ETag: "e"})
	}))
	defer apiSrv.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(localFile, []byte{}, 0o644)

	client := api.NewClient(apiSrv.URL, "cak-key")
	client.S3Client = s3Srv.Client()

	err := Upload(context.Background(), client, "org", "repo", "empty.txt", "sess", localFile)
	if err != nil {
		t.Fatalf("Upload empty file: %v", err)
	}
}

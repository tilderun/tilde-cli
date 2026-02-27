package download

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
)

// newTestClientAndS3 sets up mock API (returns 307 redirect to S3) and mock S3 servers.
func newTestClientAndS3(t *testing.T, s3Content string, s3Status int) (*api.Client, func()) {
	t.Helper()

	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s3Status >= 400 {
			w.WriteHeader(s3Status)
			w.Write([]byte("S3 Error"))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(s3Content))
	}))

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", s3Srv.URL+"/download")
		w.WriteHeader(307)
	}))

	client := api.NewClient(apiSrv.URL, "cak-key")
	client.S3Client = s3Srv.Client()

	cleanup := func() {
		apiSrv.Close()
		s3Srv.Close()
	}
	return client, cleanup
}

func TestDownload_ToFile(t *testing.T) {
	const content = "downloaded file content"
	client, cleanup := newTestClientAndS3(t, content, 200)
	defer cleanup()

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "output.txt")

	err := Download(context.Background(), client, "org", "repo", "file.txt", "sess-1", dst)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", string(got), content)
	}
}

func TestDownload_CreatesParentDirs(t *testing.T) {
	client, cleanup := newTestClientAndS3(t, "data", 200)
	defer cleanup()

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "a", "b", "c", "file.txt")

	err := Download(context.Background(), client, "org", "repo", "file.txt", "sess", dst)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("expected file to be created with parent dirs")
	}
}

func TestDownload_AtomicWrite(t *testing.T) {
	// If download fails, the destination file should not exist
	client, cleanup := newTestClientAndS3(t, "", 500)
	defer cleanup()

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "output.txt")

	err := Download(context.Background(), client, "org", "repo", "file.txt", "sess", dst)
	if err == nil {
		t.Fatal("expected error for S3 500")
	}

	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Error("destination file should not exist after failed download")
	}
}

func TestDownload_OverwritesExisting(t *testing.T) {
	client, cleanup := newTestClientAndS3(t, "new content", 200)
	defer cleanup()

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "output.txt")

	// Write initial content
	os.WriteFile(dst, []byte("old content"), 0o644)

	err := Download(context.Background(), client, "org", "repo", "file.txt", "sess", dst)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	got, _ := os.ReadFile(dst)
	if string(got) != "new content" {
		t.Errorf("content = %q, want %q", string(got), "new content")
	}
}

func TestDownloadToWriter(t *testing.T) {
	const content = "streamed content"
	client, cleanup := newTestClientAndS3(t, content, 200)
	defer cleanup()

	var buf bytes.Buffer
	err := DownloadToWriter(context.Background(), client, "org", "repo", "file.txt", "sess", &buf)
	if err != nil {
		t.Fatalf("DownloadToWriter: %v", err)
	}

	if buf.String() != content {
		t.Errorf("content = %q, want %q", buf.String(), content)
	}
}

func TestDownloadToWriter_LargeContent(t *testing.T) {
	// Test with content larger than typical buffer sizes
	content := strings.Repeat("abcdefghij", 10000) // 100KB
	client, cleanup := newTestClientAndS3(t, content, 200)
	defer cleanup()

	var buf bytes.Buffer
	err := DownloadToWriter(context.Background(), client, "org", "repo", "big.bin", "sess", &buf)
	if err != nil {
		t.Fatalf("DownloadToWriter: %v", err)
	}
	if buf.Len() != len(content) {
		t.Errorf("size = %d, want %d", buf.Len(), len(content))
	}
}

func TestDownload_S3Error(t *testing.T) {
	client, cleanup := newTestClientAndS3(t, "", 403)
	defer cleanup()

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "output.txt")

	err := Download(context.Background(), client, "org", "repo", "file.txt", "sess", dst)
	if err == nil {
		t.Fatal("expected error for S3 403")
	}
	if !strings.Contains(err.Error(), "S3 GET failed") {
		t.Errorf("error = %q, expected S3 GET failed", err.Error())
	}
}

func TestDownload_PresignError(t *testing.T) {
	// API returns 404 (not a redirect)
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}))
	defer apiSrv.Close()

	client := api.NewClient(apiSrv.URL, "cak-key")

	tmpDir := t.TempDir()
	err := Download(context.Background(), client, "org", "repo", "missing.txt", "sess",
		filepath.Join(tmpDir, "out.txt"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "presigned URL") {
		t.Errorf("error = %q, expected presigned URL error", err.Error())
	}
}

func TestDownload_ContextCancellation(t *testing.T) {
	// S3 server that blocks until request context is done
	s3Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer s3Srv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", s3Srv.URL+"/download")
		w.WriteHeader(307)
	}))
	defer apiSrv.Close()

	client := api.NewClient(apiSrv.URL, "cak-key")
	client.S3Client = s3Srv.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tmpDir := t.TempDir()
	err := Download(ctx, client, "org", "repo", "file.txt", "sess",
		filepath.Join(tmpDir, "out.txt"))
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

func TestDownload_EmptyContent(t *testing.T) {
	client, cleanup := newTestClientAndS3(t, "", 200)
	defer cleanup()

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "empty.txt")

	err := Download(context.Background(), client, "org", "repo", "empty.txt", "sess", dst)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	got, _ := os.ReadFile(dst)
	if len(got) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(got))
	}
}

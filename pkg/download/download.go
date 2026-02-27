package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
)

// Download fetches an object via presigned URL and writes it to localPath.
// If localPath is "-", it writes to the provided writer (typically stdout).
func Download(ctx context.Context, client *api.Client, org, repo, objPath, sessionID, localPath string) error {
	presignedURL, err := client.GetObjectPresignedURL(ctx, org, repo, objPath, sessionID)
	if err != nil {
		return fmt.Errorf("getting presigned URL for %s: %w", objPath, err)
	}

	return downloadFromS3(ctx, client.S3Client, presignedURL, localPath)
}

// DownloadToWriter fetches an object via presigned URL and writes it to w.
func DownloadToWriter(ctx context.Context, client *api.Client, org, repo, objPath, sessionID string, w io.Writer) error {
	presignedURL, err := client.GetObjectPresignedURL(ctx, org, repo, objPath, sessionID)
	if err != nil {
		return fmt.Errorf("getting presigned URL for %s: %w", objPath, err)
	}

	return downloadFromS3ToWriter(ctx, client.S3Client, presignedURL, w)
}

func downloadFromS3(ctx context.Context, s3Client *http.Client, url, localPath string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Write to temp file first, then rename for atomicity
	tmp, err := os.CreateTemp(dir, ".cerebral-download-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	if err := downloadFromS3ToWriter(ctx, s3Client, url, tmp); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, localPath); err != nil {
		return fmt.Errorf("renaming temp file to %s: %w", localPath, err)
	}
	return nil
}

func downloadFromS3ToWriter(ctx context.Context, s3Client *http.Client, url string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := s3Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("S3 GET failed (HTTP %d): %s", resp.StatusCode, string(errBody))
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	return nil
}

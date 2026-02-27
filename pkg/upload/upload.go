package upload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
)

// Upload performs a presigned upload pipeline: stage -> S3 PUT -> finalize.
func Upload(ctx context.Context, client *api.Client, org, repo, objPath, sessionID, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening file %s: %w", localPath, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", localPath, err)
	}

	return UploadReader(ctx, client, org, repo, objPath, sessionID, f, stat.Size(), detectContentType(localPath))
}

// UploadReader performs a presigned upload from an io.Reader.
func UploadReader(ctx context.Context, client *api.Client, org, repo, objPath, sessionID string, r io.Reader, size int64, contentType string) error {
	// Step 1: Stage — get presigned URL
	stage, err := client.StageObject(ctx, org, repo, objPath, sessionID)
	if err != nil {
		return fmt.Errorf("staging %s: %w", objPath, err)
	}

	// Step 2: PUT to S3 presigned URL
	if err := putToS3(ctx, client.S3Client, stage.UploadURL, r, size, contentType); err != nil {
		return fmt.Errorf("uploading %s to S3: %w", objPath, err)
	}

	// Step 3: Finalize
	_, err = client.FinalizeObject(ctx, org, repo, objPath, sessionID,
		stage.ExpiresAt.Format(time.RFC3339),
		&api.FinalizeRequest{
			PhysicalAddress: stage.PhysicalAddress,
			Signature:       stage.Signature,
			ContentType:     contentType,
		})
	if err != nil {
		return fmt.Errorf("finalizing %s: %w", objPath, err)
	}

	return nil
}

func putToS3(ctx context.Context, s3Client *http.Client, uploadURL string, body io.Reader, size int64, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.ContentLength = size

	resp, err := s3Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("S3 PUT failed (HTTP %d): %s", resp.StatusCode, string(errBody))
	}
	return nil
}

func detectContentType(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:n])
}

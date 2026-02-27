package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cerebral-storage/cerebral-cli/pkg/api"
	"github.com/cerebral-storage/cerebral-cli/pkg/download"
	"github.com/cerebral-storage/cerebral-cli/pkg/pool"
	"github.com/cerebral-storage/cerebral-cli/pkg/upload"
	"github.com/cerebral-storage/cerebral-cli/pkg/uri"
	"github.com/spf13/cobra"
)

func newCpCmd() *cobra.Command {
	var (
		sessionID string
		recursive bool
		verbose   bool
	)

	cmd := &cobra.Command{
		Use:   "cp [--recursive] --session ID <src> <dst>",
		Short: "Copy objects to/from a Cerebral repository",
		Long: `Copy files between a local filesystem and a Cerebral repository.
Exactly one of src or dst must be a cb:// URI.

Examples:
  cerebral cp --session ID ./file.txt cb://organization/repository/file.txt   # upload
  cerebral cp --session ID cb://organization/repository/file.txt ./file.txt   # download
  cerebral cp --session ID cb://organization/repository/file.txt -            # download to stdout
  cerebral cp --session ID -r ./data/ cb://organization/repository/data/      # recursive upload
  cerebral cp --session ID -r cb://organization/repository/data/ ./data/      # recursive download`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}

			src, dst := args[0], args[1]
			srcIsURI := uri.IsURI(src)
			dstIsURI := uri.IsURI(dst)

			if srcIsURI == dstIsURI {
				if srcIsURI {
					return fmt.Errorf("both src and dst are cb:// URIs; exactly one must be a cb:// URI")
				}
				return fmt.Errorf("neither src nor dst is a cb:// URI; exactly one must be a cb:// URI")
			}

			if srcIsURI {
				return runDownload(cmd, src, dst, sessionID, recursive, verbose)
			}
			return runUpload(cmd, src, dst, sessionID, recursive, verbose)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID (required)")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Copy recursively")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show progress for each file")
	return cmd
}

func runDownload(cmd *cobra.Command, src, dst, sessionID string, recursive, verbose bool) error {
	u, err := uri.Parse(src)
	if err != nil {
		return err
	}

	if recursive {
		return runRecursiveDownload(cmd, u, dst, sessionID, verbose)
	}

	// Single file download
	if verbose {
		fmt.Fprintf(os.Stderr, "download: %s -> %s\n", u.Path, dst)
	}
	if dst == "-" {
		return download.DownloadToWriter(cmd.Context(), apiClient, u.Org, u.Repo, u.Path, sessionID, os.Stdout)
	}

	return download.Download(cmd.Context(), apiClient, u.Org, u.Repo, u.Path, sessionID, dst)
}

func runRecursiveDownload(cmd *cobra.Command, u uri.Parsed, dst, sessionID string, verbose bool) error {
	prefix := u.Path
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Collect all objects under prefix
	var objects []string
	params := api.ListObjectsParams{
		SessionID: sessionID,
		Prefix:    prefix,
		Amount:    1000,
	}
	for {
		resp, err := apiClient.ListObjects(cmd.Context(), u.Org, u.Repo, params)
		if err != nil {
			return fmt.Errorf("listing objects: %w", err)
		}
		for _, entry := range resp.Results {
			if entry.Type != "prefix" {
				objects = append(objects, entry.Path)
			}
		}
		if !resp.Pagination.HasMore {
			break
		}
		params.After = resp.Pagination.NextOffset
	}

	if len(objects) == 0 {
		fmt.Fprintf(os.Stderr, "No objects found under %s\n", u.String())
		return nil
	}

	p := pool.New(cmd.Context(), maxConcurrency)
	var (
		mu        sync.Mutex
		failed    int
		succeeded int
	)

	for _, objPath := range objects {
		objPath := objPath
		relPath := strings.TrimPrefix(objPath, prefix)
		localPath := filepath.Join(dst, relPath)

		p.Submit(func(ctx context.Context) error {
			if verbose {
				fmt.Fprintf(os.Stderr, "download: %s -> %s\n", objPath, localPath)
			}
			if err := download.Download(ctx, apiClient, u.Org, u.Repo, objPath, sessionID, localPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error downloading %s: %s\n", objPath, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return nil // don't fail-fast on individual file errors
			}
			mu.Lock()
			succeeded++
			mu.Unlock()
			return nil
		})
	}

	_ = p.Wait()

	if failed > 0 {
		return fmt.Errorf("%d of %d files failed to download", failed, succeeded+failed)
	}
	fmt.Fprintf(os.Stderr, "Downloaded %d files.\n", succeeded)
	return nil
}

func runUpload(cmd *cobra.Command, src, dst, sessionID string, recursive, verbose bool) error {
	u, err := uri.Parse(dst)
	if err != nil {
		return err
	}

	if recursive {
		return runRecursiveUpload(cmd, src, u, sessionID, verbose)
	}

	// Single file upload
	if verbose {
		fmt.Fprintf(os.Stderr, "upload: %s -> %s\n", src, u.Path)
	}
	return upload.Upload(cmd.Context(), apiClient, u.Org, u.Repo, u.Path, sessionID, src)
}

func runRecursiveUpload(cmd *cobra.Command, src string, u uri.Parsed, sessionID string, verbose bool) error {
	prefix := u.Path
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Normalize source directory
	src = strings.TrimRight(src, "/")

	p := pool.New(cmd.Context(), maxConcurrency)
	var (
		mu        sync.Mutex
		failed    int
		succeeded int
	)

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(src, path)
		objPath := prefix + filepath.ToSlash(relPath)

		p.Submit(func(ctx context.Context) error {
			if verbose {
				fmt.Fprintf(os.Stderr, "upload: %s -> %s\n", path, objPath)
			}
			if err := upload.Upload(ctx, apiClient, u.Org, u.Repo, objPath, sessionID, path); err != nil {
				fmt.Fprintf(os.Stderr, "Error uploading %s: %s\n", path, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return nil
			}
			mu.Lock()
			succeeded++
			mu.Unlock()
			return nil
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking directory %s: %w", src, err)
	}

	_ = p.Wait()

	if failed > 0 {
		return fmt.Errorf("%d of %d files failed to upload", failed, succeeded+failed)
	}
	fmt.Fprintf(os.Stderr, "Uploaded %d files.\n", succeeded)
	return nil
}

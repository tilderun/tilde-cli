package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const githubReleasesURL = "https://api.github.com/repos/tilderun/tilde-cli/releases/latest"

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update tilde to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			current := strings.TrimPrefix(Version, "v")

			fmt.Fprintln(cmd.OutOrStdout(), "Checking for updates...")
			latest, err := fetchLatestVersion()
			if err != nil {
				return fmt.Errorf("checking latest version: %w", err)
			}

			if current == latest {
				fmt.Fprintf(cmd.OutOrStdout(), "Already up to date (%s).\n", current)
				return nil
			}

			if current == "dev" {
				fmt.Fprintln(cmd.OutOrStdout(), "Running a dev build, updating to latest release...")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Updating %s -> %s\n", current, latest)
			}

			self, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locating current binary: %w", err)
			}
			self, err = filepath.EvalSymlinks(self)
			if err != nil {
				return fmt.Errorf("resolving binary path: %w", err)
			}

			if err := downloadAndReplace(latest, self); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated to %s.\n", latest)
			return nil
		},
	}
	return cmd
}

func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", githubReleasesURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "tilde-cli/"+Version)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return strings.TrimPrefix(rel.TagName, "v"), nil
}

func downloadAndReplace(ver, destPath string) error {
	filename := fmt.Sprintf("tilde-cli_%s_%s_%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/tilderun/tilde-cli/releases/download/v%s/%s", ver, filename)

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	req.Header.Set("User-Agent", "tilde-cli/"+Version)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading archive: %w", err)
		}
		if filepath.Base(hdr.Name) != "tilde" || hdr.Typeflag != tar.TypeReg {
			continue
		}

		tmpPath := destPath + ".tmp"
		f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("writing binary: %w", err)
		}
		f.Close()

		if err := os.Rename(tmpPath, destPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("replacing binary: %w", err)
		}
		return nil
	}
	return fmt.Errorf("tilde binary not found in archive")
}

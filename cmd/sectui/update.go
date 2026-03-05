package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	githubRepo   = "orlandobianco/SecTUI"
	githubAPIURL = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func newUpdateCmd() *cobra.Command {
	var flagCheck bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update SecTUI to the latest version",
		Long: `Check for and install the latest SecTUI release from GitHub.

Examples:
  sectui update          Download and install the latest version
  sectui update --check  Only check if a newer version is available`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagCheck {
				return executeUpdateCheck()
			}
			return executeUpdate()
		},
	}

	cmd.Flags().BoolVar(&flagCheck, "check", false, "Only check for updates, don't install")

	return cmd
}

func executeUpdateCheck() error {
	fmt.Printf("%s Checking for updates...\n", bold("SecTUI"))

	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(Version, "v")

	if latest == current || Version == "dev" {
		if Version == "dev" {
			fmt.Printf("  Running development build. Latest release: %s\n", c(ansiCyan, release.TagName))
		} else {
			fmt.Printf("  %s You're on the latest version (%s)\n", c(ansiGreen+ansiBold, "✓"), Version)
		}
		return nil
	}

	fmt.Printf("  %s New version available: %s -> %s\n",
		c(ansiYellow+ansiBold, "⬆"),
		c(ansiRed, Version),
		c(ansiGreen+ansiBold, release.TagName),
	)
	fmt.Printf("  Run %s to install\n", c(ansiCyan, "sudo sectui update"))
	return nil
}

func executeUpdate() error {
	fmt.Printf("%s Checking for updates...\n\n", bold("SecTUI"))

	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(Version, "v")

	if latest == current && Version != "dev" {
		fmt.Printf("  %s Already on the latest version (%s)\n", c(ansiGreen+ansiBold, "✓"), Version)
		return nil
	}

	if Version != "dev" {
		fmt.Printf("  Updating: %s -> %s\n\n", c(ansiRed, Version), c(ansiGreen+ansiBold, release.TagName))
	} else {
		fmt.Printf("  Installing release: %s\n\n", c(ansiGreen+ansiBold, release.TagName))
	}

	// Find the right asset for this platform.
	assetName := buildAssetName()
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		fmt.Printf("  Available assets:\n")
		for _, a := range release.Assets {
			fmt.Printf("    - %s\n", a.Name)
		}
		return fmt.Errorf("no binary found for %s/%s (expected %q)", runtime.GOOS, runtime.GOARCH, assetName)
	}

	// Get the current binary path.
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %w", err)
	}

	// Check write permission.
	if err := checkWritable(execPath); err != nil {
		return fmt.Errorf("cannot write to %s: %w\nRun with: sudo sectui update", execPath, err)
	}

	fmt.Printf("  Downloading %s...\n", assetName)

	// Download to temp file.
	tmpFile, err := os.CreateTemp("", "sectui-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := httpGet(downloadURL)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	written, err := io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Printf("  Downloaded %d bytes\n", written)

	// Make executable.
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Replace current binary.
	fmt.Printf("  Replacing %s...\n", execPath)
	if err := os.Rename(tmpPath, execPath); err != nil {
		// Fallback: copy if rename fails (cross-device).
		data, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return fmt.Errorf("replace failed: %w", err)
		}
		if writeErr := os.WriteFile(execPath, data, 0o755); writeErr != nil {
			return fmt.Errorf("replace failed: %w", writeErr)
		}
	}

	fmt.Printf("\n  %s SecTUI updated to %s\n", c(ansiGreen+ansiBold, "✓"), c(ansiGreen+ansiBold, release.TagName))
	return nil
}

func fetchLatestRelease() (*githubRelease, error) {
	resp, err := httpGet(githubAPIURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &release, nil
}

func httpGet(url string) (*http.Response, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SecTUI/"+Version)
	req.Header.Set("Accept", "application/octet-stream")
	return client.Do(req)
}

func buildAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	}
	return fmt.Sprintf("sectui_%s_%s", os, arch)
}

func checkWritable(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	f.Close()
	return nil
}

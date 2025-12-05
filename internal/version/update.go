package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// UpdateInfo contains information about available updates.
type UpdateInfo struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	Error           error
}

// GitHubRelease represents the GitHub API response for releases.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
}

// CheckForUpdate checks GitHub for the latest release version.
func CheckForUpdate() UpdateInfo {
	info := UpdateInfo{
		CurrentVersion: Version,
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Use releases/latest endpoint for the most recent release
	resp, err := client.Get("https://api.github.com/repos/litescript/ls-horizons/releases/latest")
	if err != nil {
		info.Error = fmt.Errorf("failed to check for updates: %w", err)
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// If no releases, try tags instead
		return checkForUpdateViaTags(info, client)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		info.Error = fmt.Errorf("failed to parse update response: %w", err)
		return info
	}

	info.LatestVersion = normalizeVersion(release.TagName)
	info.UpdateAvailable = isNewerVersion(info.LatestVersion, info.CurrentVersion)

	return info
}

// checkForUpdateViaTags falls back to checking tags if no releases exist.
func checkForUpdateViaTags(info UpdateInfo, client *http.Client) UpdateInfo {
	resp, err := client.Get("https://api.github.com/repos/litescript/ls-horizons/tags")
	if err != nil {
		info.Error = fmt.Errorf("failed to check for updates: %w", err)
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		info.Error = fmt.Errorf("failed to check for updates: status %d", resp.StatusCode)
		return info
	}

	var tags []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		info.Error = fmt.Errorf("failed to parse update response: %w", err)
		return info
	}

	if len(tags) == 0 {
		info.LatestVersion = info.CurrentVersion
		return info
	}

	// Tags are returned newest first
	info.LatestVersion = normalizeVersion(tags[0].TagName)
	info.UpdateAvailable = isNewerVersion(info.LatestVersion, info.CurrentVersion)

	return info
}

// normalizeVersion strips the "v" prefix if present.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// isNewerVersion returns true if latest is newer than current.
func isNewerVersion(latest, current string) bool {
	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		var latestNum, currentNum int
		fmt.Sscanf(latestParts[i], "%d", &latestNum)
		fmt.Sscanf(currentParts[i], "%d", &currentNum)

		if latestNum > currentNum {
			return true
		} else if latestNum < currentNum {
			return false
		}
	}

	return len(latestParts) > len(currentParts)
}

// InstallCommand returns the command to update the application.
func InstallCommand() string {
	return "go install github.com/litescript/ls-horizons/cmd/ls-horizons@latest"
}

package git

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// GetProjectName returns the project name from git remote or directory name
func GetProjectName() (string, error) {
	// Try to get from git remote
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err == nil {
		return parseProjectNameFromRemote(string(output)), nil
	}

	// Fall back to directory name
	return getDirectoryName()
}

// parseProjectNameFromRemote extracts project name from a git remote URL
func parseProjectNameFromRemote(remoteURL string) string {
	remoteURL = strings.TrimSpace(remoteURL)

	// Handle SSH URLs: git@github.com:user/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		parts := strings.Split(remoteURL, ":")
		if len(parts) == 2 {
			remoteURL = parts[1]
		}
	}

	// Handle HTTPS URLs: https://github.com/user/repo.git
	if strings.Contains(remoteURL, "/") {
		parts := strings.Split(remoteURL, "/")
		remoteURL = parts[len(parts)-1]
	}

	// Remove .git suffix
	remoteURL = strings.TrimSuffix(remoteURL, ".git")

	return remoteURL
}

// getDirectoryName returns the current directory name
func getDirectoryName() (string, error) {
	cmd := exec.Command("pwd")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	dir := strings.TrimSpace(string(output))
	return filepath.Base(dir), nil
}

// GetRepoRoot returns the root directory of the git repository
func GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

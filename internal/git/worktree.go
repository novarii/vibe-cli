package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/novari/vibe-cli/internal/config"
)

// Worktree represents a git worktree
type Worktree struct {
	Path   string
	Branch string
	Head   string
}

// GetWorktreePath returns the path for a worktree given project and feature
func GetWorktreePath(project, feature string) (string, error) {
	repoRoot, err := GetRepoRoot()
	if err != nil {
		return "", err
	}

	// Worktrees are stored relative to repo root's parent
	parentDir := filepath.Dir(repoRoot)
	return filepath.Join(parentDir, "worktrees", project, feature), nil
}

// CreateWorktree creates a new git worktree
func CreateWorktree(path, branch, base string) error {
	// Ensure parent directory exists
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create the worktree with a new branch
	cmd := exec.Command("git", "worktree", "add", "-b", branch, path, base)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	return nil
}

// RemoveWorktree removes a git worktree
func RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove", path}
	if force {
		args = append(args, "--force")
	}

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if force {
			// If git worktree remove fails even with force, manually delete and prune
			fmt.Printf("Git worktree remove failed, forcing manual cleanup...\n")

			// Remove directory
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to remove worktree directory: %w", err)
			}

			// Prune worktree list
			pruneCmd := exec.Command("git", "worktree", "prune")
			pruneCmd.Stdout = os.Stdout
			pruneCmd.Stderr = os.Stderr
			pruneCmd.Run() // Ignore error, best effort

			return nil
		}
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	return nil
}

// DeleteBranch deletes a git branch
func DeleteBranch(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}

	cmd := exec.Command("git", "branch", flag, branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	return nil
}

// ListWorktrees returns all worktrees for the current repository
func ListWorktrees() ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current Worktree

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "worktree ") {
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		} else if strings.HasPrefix(line, "HEAD ") {
			current.Head = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}

	// Don't forget the last one
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// WorktreeExists checks if a worktree exists at the given path
func WorktreeExists(path string) bool {
	worktrees, err := ListWorktrees()
	if err != nil {
		return false
	}

	for _, wt := range worktrees {
		if wt.Path == path {
			return true
		}
	}

	return false
}

// GetFeatureWorktrees returns worktrees that match the feature branch pattern
func GetFeatureWorktrees(project string) ([]Worktree, error) {
	worktrees, err := ListWorktrees()
	if err != nil {
		return nil, err
	}

	var featureWorktrees []Worktree
	for _, wt := range worktrees {
		// Check if this is a feature branch
		if strings.HasPrefix(wt.Branch, "feature/") {
			// Extract feature name from path
			expectedPath, _ := GetWorktreePath(project, strings.TrimPrefix(wt.Branch, "feature/"))
			if wt.Path == expectedPath {
				featureWorktrees = append(featureWorktrees, wt)
			}
		}
	}

	return featureWorktrees, nil
}

// FeatureFromBranch extracts the feature name from a branch name
func FeatureFromBranch(branch string) string {
	return strings.TrimPrefix(branch, config.BranchName(""))
}

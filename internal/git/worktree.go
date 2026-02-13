package git

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
)

// Worktree represents a git worktree
type Worktree struct {
	Path   string
	Branch string
	Head   string
}

// ContainerGit runs git operations inside a container via docker exec
type ContainerGit struct {
	Docker        *docker.Client
	ContainerName string
}

// CreateWorktree creates a new git worktree inside the container
func (cg *ContainerGit) CreateWorktree(feature, base string) error {
	branch := config.BranchName(feature)
	wtPath := config.WorktreePath(feature)

	cmd := []string{
		"git", "-C", config.DefaultRepoMount,
		"worktree", "add", "-b", branch, wtPath, base,
	}

	exitCode, output, err := cg.Docker.ExecNonInteractive(cg.ContainerName, cmd)
	if err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("failed to create worktree: %s", strings.TrimSpace(output))
	}

	return nil
}

// RemoveWorktree removes a git worktree inside the container
func (cg *ContainerGit) RemoveWorktree(feature string, force bool) error {
	wtPath := config.WorktreePath(feature)

	args := []string{
		"git", "-C", config.DefaultRepoMount,
		"worktree", "remove", wtPath,
	}
	if force {
		args = append(args, "--force")
	}

	exitCode, output, err := cg.Docker.ExecNonInteractive(cg.ContainerName, args)
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}
	if exitCode != 0 {
		if force {
			// Fall back to rm -rf + prune
			rmCmd := []string{"rm", "-rf", wtPath}
			exitCode, output, err = cg.Docker.ExecNonInteractive(cg.ContainerName, rmCmd)
			if err != nil {
				return fmt.Errorf("failed to remove worktree directory: %w", err)
			}
			if exitCode != 0 {
				return fmt.Errorf("failed to remove worktree directory: %s", strings.TrimSpace(output))
			}

			pruneCmd := []string{"git", "-C", config.DefaultRepoMount, "worktree", "prune"}
			cg.Docker.ExecNonInteractive(cg.ContainerName, pruneCmd) // best effort
			return nil
		}
		return fmt.Errorf("failed to remove worktree: %s", strings.TrimSpace(output))
	}

	return nil
}

// DeleteBranch deletes a git branch inside the container
func (cg *ContainerGit) DeleteBranch(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}

	cmd := []string{
		"git", "-C", config.DefaultRepoMount,
		"branch", flag, branch,
	}

	exitCode, output, err := cg.Docker.ExecNonInteractive(cg.ContainerName, cmd)
	if err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("failed to delete branch: %s", strings.TrimSpace(output))
	}

	return nil
}

// ListWorktrees returns worktrees under /worktrees/ inside the container
func (cg *ContainerGit) ListWorktrees() ([]Worktree, error) {
	cmd := []string{
		"git", "-C", config.DefaultRepoMount,
		"worktree", "list", "--porcelain",
	}

	exitCode, output, err := cg.Docker.ExecNonInteractive(cg.ContainerName, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("failed to list worktrees: %s", strings.TrimSpace(output))
	}

	var worktrees []Worktree
	var current Worktree

	scanner := bufio.NewScanner(strings.NewReader(output))
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

	// Filter to only worktrees under /worktrees/
	var filtered []Worktree
	for _, wt := range worktrees {
		if strings.HasPrefix(wt.Path, config.DefaultWorktreeBase+"/") {
			filtered = append(filtered, wt)
		}
	}

	return filtered, nil
}

// WorktreeExists checks if a feature worktree exists inside the container
func (cg *ContainerGit) WorktreeExists(feature string) (bool, error) {
	wtPath := config.WorktreePath(feature)
	cmd := []string{"test", "-d", wtPath}

	exitCode, _, err := cg.Docker.ExecNonInteractive(cg.ContainerName, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check worktree: %w", err)
	}

	return exitCode == 0, nil
}

// FeatureFromBranch extracts the feature name from a branch name
func FeatureFromBranch(branch string) string {
	return strings.TrimPrefix(branch, config.BranchName(""))
}

package cli

import (
	"fmt"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var (
	forceCleanup bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup <feature>",
	Short: "Remove worktree, branch, and Docker container",
	Long: `Removes the git worktree, associated branch, and Docker container
for the specified feature.

Use --force to force removal of a worktree with uncommitted changes.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runCleanup,
	Example: `  vibe cleanup auth          # Cleanup auth feature
  vibe cleanup auth --force  # Force cleanup with uncommitted changes`,
}

func init() {
	cleanupCmd.Flags().BoolVarP(&forceCleanup, "force", "f", false, "Force removal even with uncommitted changes")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	feature := args[0]

	// Get project name
	project, err := git.GetProjectName()
	if err != nil {
		return fmt.Errorf("failed to detect project name: %w", err)
	}

	fmt.Printf("Project: %s\n", project)
	fmt.Printf("Feature: %s\n", feature)

	// Get worktree path
	worktreePath, err := git.GetWorktreePath(project, feature)
	if err != nil {
		return fmt.Errorf("failed to get worktree path: %w", err)
	}

	// Remove Docker container
	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	containerName := config.ContainerName(project, feature)
	if err := dockerClient.CleanupContainer(containerName); err != nil {
		fmt.Printf("Warning: failed to cleanup container: %v\n", err)
	}

	// Remove worktree
	if git.WorktreeExists(worktreePath) {
		fmt.Printf("Removing worktree at %s...\n", worktreePath)
		if err := git.RemoveWorktree(worktreePath, forceCleanup); err != nil {
			return err
		}
	} else {
		fmt.Printf("Worktree does not exist at %s\n", worktreePath)
	}

	// Delete branch
	branch := config.BranchName(feature)
	fmt.Printf("Deleting branch %s...\n", branch)
	if err := git.DeleteBranch(branch, forceCleanup); err != nil {
		fmt.Printf("Warning: failed to delete branch: %v\n", err)
	}

	fmt.Printf("\nCleanup complete for feature '%s'\n", feature)
	return nil
}

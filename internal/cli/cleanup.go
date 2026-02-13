package cli

import (
	"fmt"
	"os/exec"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var (
	forceCleanup    bool
	removeContainer bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup <feature> [feature...]",
	Short: "Remove worktrees and branches inside the container",
	Long: `Removes git worktrees and associated branches inside the Docker container
for the specified features.

Use --force to force removal of worktrees with uncommitted changes.
Use --remove-container to also stop and remove the Docker container.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCleanup,
	Example: `  vibe cleanup auth                    # Cleanup auth feature
  vibe cleanup auth payments           # Cleanup multiple features
  vibe cleanup auth --force            # Force cleanup with uncommitted changes
  vibe cleanup auth --remove-container # Also remove the container`,
}

func init() {
	cleanupCmd.Flags().BoolVarP(&forceCleanup, "force", "f", false, "Force removal even with uncommitted changes")
	cleanupCmd.Flags().BoolVar(&removeContainer, "remove-container", false, "Also stop and remove the Docker container")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	features := args

	// Get project name
	project, err := git.GetProjectName()
	if err != nil {
		return fmt.Errorf("failed to detect project name: %w", err)
	}

	// Resolve container name
	containerName := resolveContainerName(project)

	fmt.Printf("Project: %s\n", project)
	fmt.Printf("Container: %s\n", containerName)

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Check if container is running for worktree operations
	if !dockerClient.ContainerRunning(containerName) {
		if removeContainer {
			// Container exists but not running — just clean it up
			if err := dockerClient.CleanupContainer(containerName); err != nil {
				fmt.Printf("Warning: failed to cleanup container: %v\n", err)
			}
			fmt.Println("\nCleanup complete.")
			return nil
		}
		return fmt.Errorf("container %s is not running — cannot remove worktrees", containerName)
	}

	// Set up ContainerGit for in-container operations
	cg := &git.ContainerGit{
		Docker:        dockerClient,
		ContainerName: containerName,
	}

	// Remove worktrees and branches for each feature
	for _, feature := range features {
		branch := config.BranchName(feature)

		fmt.Printf("\nCleaning up feature '%s'...\n", feature)

		// Remove worktree
		fmt.Printf("  Removing worktree %s...\n", config.WorktreePath(feature))
		if err := cg.RemoveWorktree(feature, forceCleanup); err != nil {
			fmt.Printf("  Warning: failed to remove worktree: %v\n", err)
		}

		// Delete branch
		fmt.Printf("  Deleting branch %s...\n", branch)
		if err := cg.DeleteBranch(branch, forceCleanup); err != nil {
			fmt.Printf("  Warning: failed to delete branch: %v\n", err)
		}
	}

	// If --remove-container, stop and remove the container, then prune stale worktree refs on host
	if removeContainer {
		fmt.Println()
		if err := dockerClient.CleanupContainer(containerName); err != nil {
			fmt.Printf("Warning: failed to cleanup container: %v\n", err)
		}

		// Prune stale worktree references on host (container worktree paths no longer exist)
		if err := exec.Command("git", "worktree", "prune").Run(); err != nil {
			fmt.Printf("Warning: failed to prune worktrees: %v\n", err)
		}
	}

	fmt.Println("\nCleanup complete.")
	return nil
}

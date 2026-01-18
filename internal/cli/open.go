package cli

import (
	"fmt"
	"os"

	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <feature>",
	Short: "Reopen an existing worktree and continue the Claude session",
	Long: `Reopens an existing git worktree and continues the previous
Claude Code session using the -c (continue) flag.

The worktree must already exist at ../worktrees/<project>/<feature>.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runOpen,
	Example: `  vibe open auth   # Continue working on auth feature`,
}

func runOpen(cmd *cobra.Command, args []string) error {
	feature := args[0]

	// Get project name
	project, err := git.GetProjectName()
	if err != nil {
		return fmt.Errorf("failed to detect project name: %w", err)
	}

	// Get repo root for git worktree support
	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return fmt.Errorf("failed to get repo root: %w", err)
	}

	fmt.Printf("Project: %s\n", project)
	fmt.Printf("Feature: %s\n", feature)

	// Get worktree path
	worktreePath, err := git.GetWorktreePath(project, feature)
	if err != nil {
		return fmt.Errorf("failed to get worktree path: %w", err)
	}

	// Check if worktree exists
	if !git.WorktreeExists(worktreePath) {
		return fmt.Errorf("worktree does not exist at %s - use 'vibe new %s' to create it", worktreePath, feature)
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Ensure container is running
	containerCfg := docker.DefaultContainerConfig(project, feature, worktreePath, repoRoot)
	if err := dockerClient.EnsureContainer(containerCfg); err != nil {
		return err
	}

	// Try to continue existing session, fall back to new session
	fmt.Printf("\nStarting Claude in container %s...\n", containerCfg.Name)
	fmt.Println("Press Ctrl+C to exit Claude and return to your shell.\n")

	// First try with continue flag
	claudeCmd := []string{"claude", "--dangerously-skip-permissions", "-c"}
	exitCode, err := dockerClient.ExecInteractive(containerCfg.Name, claudeCmd)

	// If continue failed (exit code 1 typically means no session), try without -c
	if exitCode != 0 {
		fmt.Println("\nNo existing session found, starting new session...")
		claudeCmd = []string{"claude", "--dangerously-skip-permissions"}
		exitCode, err = dockerClient.ExecInteractive(containerCfg.Name, claudeCmd)
	}

	if err != nil {
		return fmt.Errorf("failed to run Claude: %w", err)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

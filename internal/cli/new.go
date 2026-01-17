package cli

import (
	"fmt"
	"os"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <feature> [base]",
	Short: "Create a new worktree and start a fresh Claude session",
	Long: `Creates a new git worktree for the specified feature and starts
a Docker-sandboxed Claude Code session.

The worktree will be created at ../worktrees/<project>/<feature>
with a new branch feature/<feature> based on the specified base branch.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runNew,
	Example: `  vibe new auth           # Create worktree based on main
  vibe new auth develop   # Create worktree based on develop`,
}

func runNew(cmd *cobra.Command, args []string) error {
	feature := args[0]
	base := "main"
	if len(args) > 1 {
		base = args[1]
	}

	// Get project name
	project, err := git.GetProjectName()
	if err != nil {
		return fmt.Errorf("failed to detect project name: %w", err)
	}

	// Get repo root for loading config and copying files
	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return fmt.Errorf("failed to get repo root: %w", err)
	}

	// Load .vibe.yaml config
	vibeCfg, err := config.LoadVibeConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to load .vibe.yaml: %w", err)
	}

	fmt.Printf("Project: %s\n", project)
	fmt.Printf("Feature: %s\n", feature)
	fmt.Printf("Base branch: %s\n", base)

	// Get worktree path
	worktreePath, err := git.GetWorktreePath(project, feature)
	if err != nil {
		return fmt.Errorf("failed to get worktree path: %w", err)
	}

	// Check if worktree already exists
	if git.WorktreeExists(worktreePath) {
		return fmt.Errorf("worktree already exists at %s - use 'vibe open %s' instead", worktreePath, feature)
	}

	// Create worktree
	branch := config.BranchName(feature)
	fmt.Printf("\nCreating worktree at %s...\n", worktreePath)
	if err := git.CreateWorktree(worktreePath, branch, base); err != nil {
		return err
	}

	// Copy files from .vibe.yaml config
	if len(vibeCfg.Copy) > 0 {
		fmt.Printf("Copying files from .vibe.yaml...\n")
		if err := vibeCfg.CopyFiles(repoRoot, worktreePath); err != nil {
			return fmt.Errorf("failed to copy files: %w", err)
		}
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Ensure container is running with env vars from config
	containerCfg := docker.DefaultContainerConfig(project, feature, worktreePath)
	containerCfg.ExtraEnv = vibeCfg.GetEnvValues()
	if err := dockerClient.EnsureContainer(containerCfg); err != nil {
		return err
	}

	// Run Claude
	fmt.Printf("\nStarting Claude in container %s...\n", containerCfg.Name)
	fmt.Println("Press Ctrl+C to exit Claude and return to your shell.\n")

	claudeCmd := []string{"claude", "--dangerously-skip-permissions"}
	exitCode, err := dockerClient.ExecInteractive(containerCfg.Name, claudeCmd)
	if err != nil {
		return fmt.Errorf("failed to run Claude: %w", err)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

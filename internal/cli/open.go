package cli

import (
	"fmt"
	"os"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <feature>",
	Short: "Reopen an existing worktree and continue the Claude session",
	Long: `Reopens an existing git worktree inside the container and continues
the previous Claude Code session using the -c (continue) flag.

The worktree must already exist at /worktrees/<feature> inside the container.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runOpen,
	Example: `  vibe open auth   # Continue working on auth feature`,
}

func runOpen(cmd *cobra.Command, args []string) error {
	feature := args[0]

	// Get project name and repo root
	project, err := git.GetProjectName()
	if err != nil {
		return fmt.Errorf("failed to detect project name: %w", err)
	}

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return fmt.Errorf("failed to get repo root: %w", err)
	}

	// Load .vibe.yaml config
	vibeCfg, err := config.LoadVibeConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to load .vibe.yaml: %w", err)
	}

	// Resolve container name
	containerName := resolveContainerName(project)

	fmt.Printf("Project: %s\n", project)
	fmt.Printf("Feature: %s\n", feature)

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Ensure container is running
	containerCfg := docker.DefaultContainerConfig(project, repoRoot)
	containerCfg.Name = containerName
	containerCfg.ExtraEnv = vibeCfg.GetEnvValues()
	containerCfg.ExtraMounts = vibeCfg.GetMounts()
	containerCfg.Network = vibeCfg.Network
	if err := dockerClient.EnsureContainer(containerCfg); err != nil {
		return err
	}

	// Check worktree exists inside container
	cg := &git.ContainerGit{
		Docker:        dockerClient,
		ContainerName: containerName,
	}

	exists, err := cg.WorktreeExists(feature)
	if err != nil {
		return fmt.Errorf("failed to check worktree: %w", err)
	}
	if !exists {
		return fmt.Errorf("worktree %s does not exist — use 'vibe new %s' to create it", feature, feature)
	}

	wtPath := config.WorktreePath(feature)

	// Try to continue existing session, fall back to new session
	fmt.Printf("\nStarting Claude in container %s...\n", containerName)
	fmt.Println("Press Ctrl+C to exit Claude and return to your shell.")

	// First try with continue flag
	claudeCmd := []string{"bash", "-c", fmt.Sprintf("cd %s && claude --dangerously-skip-permissions -c", wtPath)}
	exitCode, err := dockerClient.ExecInteractive(containerName, claudeCmd)

	// If continue failed (exit code 1 typically means no session), try without -c
	if exitCode != 0 {
		fmt.Println("\nNo existing session found, starting new session...")
		claudeCmd = []string{"bash", "-c", fmt.Sprintf("cd %s && claude --dangerously-skip-permissions", wtPath)}
		exitCode, err = dockerClient.ExecInteractive(containerName, claudeCmd)
	}

	if err != nil {
		return fmt.Errorf("failed to run Claude: %w", err)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

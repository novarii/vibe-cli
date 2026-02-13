package cli

import (
	"fmt"
	"os"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var newBaseFlag string

var newCmd = &cobra.Command{
	Use:   "new <feature> [feature...]",
	Short: "Create worktrees and start Claude sessions in the container",
	Long: `Creates git worktrees inside the project's Docker container for the
specified features and optionally starts a Claude Code session.

Each feature gets a worktree at /worktrees/<feature> with a new branch
feature/<feature> based on the specified base branch.

With a single feature, an interactive Claude session is launched.
With multiple features, worktrees are created and a summary is printed.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runNew,
	Example: `  vibe new auth                  # Create worktree, launch Claude
  vibe new auth payments api     # Create multiple worktrees
  vibe new auth --base develop   # Base on develop branch`,
}

func init() {
	newCmd.Flags().StringVar(&newBaseFlag, "base", "main", "Base branch for new worktrees")
}

func runNew(cmd *cobra.Command, args []string) error {
	features := args

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
	fmt.Printf("Container: %s\n", containerName)
	fmt.Printf("Base branch: %s\n", newBaseFlag)

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Ensure container is running with repo mounted at /repo
	containerCfg := docker.DefaultContainerConfig(project, repoRoot)
	containerCfg.Name = containerName
	containerCfg.ExtraEnv = vibeCfg.GetEnvValues()
	containerCfg.ExtraMounts = vibeCfg.GetMounts()
	containerCfg.Network = vibeCfg.Network
	if err := dockerClient.EnsureContainer(containerCfg); err != nil {
		return err
	}

	// Set up ContainerGit for in-container operations
	cg := &git.ContainerGit{
		Docker:        dockerClient,
		ContainerName: containerName,
	}

	// Create worktrees for each feature
	for _, feature := range features {
		// Check if worktree already exists
		exists, err := cg.WorktreeExists(feature)
		if err != nil {
			return fmt.Errorf("failed to check worktree for %s: %w", feature, err)
		}
		if exists {
			fmt.Printf("Worktree %s already exists — skipping (use 'vibe open %s' instead)\n", feature, feature)
			continue
		}

		fmt.Printf("\nCreating worktree for %s...\n", feature)
		if err := cg.CreateWorktree(feature, newBaseFlag); err != nil {
			return err
		}

		// Copy files from .vibe.yaml config
		if len(vibeCfg.Copy) > 0 {
			fmt.Printf("Copying files for %s...\n", feature)
			if err := vibeCfg.CopyFilesInContainer(dockerClient, containerName, feature); err != nil {
				return fmt.Errorf("failed to copy files for %s: %w", feature, err)
			}
		}
	}

	// Single feature: launch interactive Claude session
	if len(features) == 1 {
		feature := features[0]
		wtPath := config.WorktreePath(feature)

		fmt.Printf("\nStarting Claude in container %s...\n", containerName)
		fmt.Println("Press Ctrl+C to exit Claude and return to your shell.")

		claudeCmd := []string{"bash", "-c", fmt.Sprintf("cd %s && claude --dangerously-skip-permissions", wtPath)}
		exitCode, err := dockerClient.ExecInteractive(containerName, claudeCmd)
		if err != nil {
			return fmt.Errorf("failed to run Claude: %w", err)
		}

		if exitCode != 0 {
			os.Exit(exitCode)
		}

		return nil
	}

	// Multiple features: print summary
	fmt.Printf("\nCreated %d worktrees:\n", len(features))
	for _, feature := range features {
		fmt.Printf("  %-20s  %s\n", feature, config.WorktreePath(feature))
	}
	fmt.Printf("\nUse 'vibe open <feature>' to start a Claude session in a worktree.\n")

	return nil
}

package cli

import (
	"fmt"
	"os"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var terminalCmd = &cobra.Command{
	Use:   "terminal <feature>",
	Short: "Open a shell in the feature's worktree",
	Long: `Opens an interactive shell (bash) in the Docker container,
starting in the specified feature's worktree directory.
Useful for running commands while Claude is still active in another terminal.`,
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"term", "sh", "shell"},
	RunE:    runTerminal,
	Example: `  vibe terminal auth
  vibe term auth
  vibe sh auth`,
}

func runTerminal(cmd *cobra.Command, args []string) error {
	feature := args[0]

	// Get project name
	project, err := git.GetProjectName()
	if err != nil {
		return fmt.Errorf("failed to detect project name: %w", err)
	}

	// Resolve container name
	containerName := resolveContainerName(project)

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Check if container is running
	if !dockerClient.ContainerRunning(containerName) {
		return fmt.Errorf("container %s is not running — use 'vibe new %s' or 'vibe open %s' first", containerName, feature, feature)
	}

	wtPath := config.WorktreePath(feature)
	fmt.Printf("Opening shell in %s at %s...\n\n", containerName, wtPath)

	// Run bash, cd to worktree directory
	shellCmd := []string{"bash", "-c", fmt.Sprintf("cd %s && exec bash", wtPath)}
	exitCode, err := dockerClient.ExecInteractive(containerName, shellCmd)
	if err != nil {
		return fmt.Errorf("failed to open shell: %w", err)
	}

	if exitCode != 0 && exitCode != 130 {
		// 130 = interrupted by Ctrl+C, which is normal
		os.Exit(exitCode)
	}

	return nil
}

package cli

import (
	"fmt"

	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the project's Docker container",
	Long: `Stops the running Docker container for the current project.
The container can be restarted later with any vibe command that needs it.`,
	Args: cobra.NoArgs,
	RunE: runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
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
		fmt.Printf("Container %s is not running.\n", containerName)
		return nil
	}

	// Stop the container
	fmt.Printf("Stopping container %s...\n", containerName)
	if err := dockerClient.StopContainer(containerName); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	fmt.Printf("Container %s stopped.\n", containerName)
	return nil
}

package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees for current project",
	Long: `Lists all feature worktrees inside the project's Docker container,
showing their branches and the container status.`,
	Args:    cobra.NoArgs,
	RunE:    runList,
	Aliases: []string{"ls"},
}

func runList(cmd *cobra.Command, args []string) error {
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

	// Get container status
	status, err := dockerClient.GetContainerStatus(containerName)
	if err != nil {
		status = "error"
	}

	fmt.Printf("PROJECT:   %s\n", project)
	fmt.Printf("CONTAINER: %s (%s)\n\n", containerName, status)

	// If container is not running, we can't list worktrees
	if !dockerClient.ContainerRunning(containerName) {
		fmt.Println("Container is not running — no worktrees to show.")
		return nil
	}

	// List worktrees inside the container
	cg := &git.ContainerGit{
		Docker:        dockerClient,
		ContainerName: containerName,
	}

	worktrees, err := cg.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		fmt.Println("No worktrees found.")
		return nil
	}

	// Set up tabwriter for nice output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FEATURE\tBRANCH\tPATH")

	for _, wt := range worktrees {
		feature := git.FeatureFromBranch(wt.Branch)
		fmt.Fprintf(w, "%s\t%s\t%s\n", feature, wt.Branch, wt.Path)
	}

	w.Flush()
	return nil
}

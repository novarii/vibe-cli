package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"os"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees for current project",
	Long:  `Lists all feature worktrees for the current project, showing their status and associated Docker containers.`,
	Args:  cobra.NoArgs,
	RunE:  runList,
	Aliases: []string{"ls"},
}

func runList(cmd *cobra.Command, args []string) error {
	// Get project name
	project, err := git.GetProjectName()
	if err != nil {
		return fmt.Errorf("failed to detect project name: %w", err)
	}

	fmt.Printf("PROJECT: %s\n\n", project)

	// Get all worktrees
	worktrees, err := git.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Set up tabwriter for nice output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FEATURE\tBRANCH\tCONTAINER\tSTATUS")

	for _, wt := range worktrees {
		// Only show feature branches
		if !strings.HasPrefix(wt.Branch, "feature/") {
			continue
		}

		feature := git.FeatureFromBranch(wt.Branch)
		containerName := config.ContainerName(project, feature)

		// Get container status
		status, err := dockerClient.GetContainerStatus(containerName)
		if err != nil {
			status = "error"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", feature, wt.Branch, containerName, status)
	}

	w.Flush()
	return nil
}

package cli

import (
	"fmt"
	"os"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	Version       = "0.1.0"
	containerFlag string
)

var rootCmd = &cobra.Command{
	Use:   "vibe",
	Short: "Manage Docker-sandboxed Claude agents with in-container git worktrees",
	Long: `Vibe mounts your repo into a single long-lived Docker container and creates
git worktrees inside it for agent isolation. Multiple agents run as separate
processes in one container, each in its own worktree.

Use --container to run multiple containers for different contexts.`,
	Version: Version,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// resolveContainerName returns the container name, using --container flag if set.
func resolveContainerName(project string) string {
	if containerFlag != "" {
		return "vibe-" + containerFlag
	}
	return config.ContainerName(project)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&containerFlag, "container", "", "Override container name suffix (default: project name)")

	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(loopCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(terminalCmd)
	rootCmd.AddCommand(stopCmd)
}

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version = "0.1.0"
)

var rootCmd = &cobra.Command{
	Use:   "vibe",
	Short: "Manage isolated development environments with git worktrees and Docker-sandboxed Claude agents",
	Long: `Vibe automates the workflow of creating isolated feature branches
with sandboxed AI agents for autonomous development loops.

It combines git worktrees for code isolation with Docker containers
running Claude Code for safe, autonomous development.`,
	Version: Version,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(loopCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(terminalCmd)
}

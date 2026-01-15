package cli

import (
	"fmt"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
	"github.com/novari/vibe-cli/internal/git"
	"github.com/novari/vibe-cli/internal/loop"
	"github.com/spf13/cobra"
)

var (
	maxIterations     int
	completionPromise string
	promptFile        string
)

var loopCmd = &cobra.Command{
	Use:   "loop <feature>",
	Short: "Run a Ralph-style loop with new Claude sessions each iteration",
	Long: `Runs an autonomous loop where Claude executes the prompt file
repeatedly until a completion condition is met.

The loop will:
1. Start a new Claude session with the prompt file
2. Capture the output
3. Check for completion promise (if specified)
4. Repeat until max iterations or completion

Exit conditions:
- Max iterations reached (--max-iterations)
- Completion promise detected (--completion-promise)
- User interrupt (Ctrl+C)
- Claude error (non-zero exit)`,
	Args: cobra.ExactArgs(1),
	RunE: runLoop,
	Example: `  vibe loop auth
  vibe loop auth --max-iterations 50
  vibe loop auth --max-iterations 50 --completion-promise "DONE"
  vibe loop auth --prompt-file custom-prompt.md`,
}

func init() {
	loopCmd.Flags().IntVar(&maxIterations, "max-iterations", config.DefaultMaxIterations, "Max iterations (0 = unlimited)")
	loopCmd.Flags().StringVar(&completionPromise, "completion-promise", "", "Stop when <promise>VALUE</promise> detected")
	loopCmd.Flags().StringVar(&promptFile, "prompt-file", config.DefaultPromptFile, "Prompt file to use")
}

func runLoop(cmd *cobra.Command, args []string) error {
	feature := args[0]

	// Get project name
	project, err := git.GetProjectName()
	if err != nil {
		return fmt.Errorf("failed to detect project name: %w", err)
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
		return fmt.Errorf("worktree does not exist at %s - use 'vibe new %s' first", worktreePath, feature)
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Display loop configuration
	fmt.Printf("\nLoop Configuration:\n")
	fmt.Printf("  Prompt file: %s\n", promptFile)
	fmt.Printf("  Max iterations: ")
	if maxIterations == 0 {
		fmt.Println("unlimited")
	} else {
		fmt.Printf("%d\n", maxIterations)
	}
	if completionPromise != "" {
		fmt.Printf("  Completion promise: <promise>%s</promise>\n", completionPromise)
	}

	// Create and run the loop
	loopCfg := loop.Config{
		Feature:           feature,
		Project:           project,
		WorktreePath:      worktreePath,
		MaxIterations:     maxIterations,
		CompletionPromise: completionPromise,
		PromptFile:        promptFile,
	}

	runner := loop.NewRunner(dockerClient, loopCfg)
	return runner.Run()
}

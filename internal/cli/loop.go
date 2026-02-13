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
	yoloMode          bool
	detectPR          bool
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
- PR created (--detect-pr, enabled by default)
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
	loopCmd.Flags().BoolVar(&yoloMode, "yolo", false, "Full auto mode: non-interactive, just prints output")
	loopCmd.Flags().BoolVar(&detectPR, "detect-pr", true, "Exit loop when PR is created (detects GitHub PR URLs)")
}

func runLoop(cmd *cobra.Command, args []string) error {
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
	fmt.Printf("Container: %s\n", containerName)

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
	if yoloMode {
		fmt.Println("  Mode: YOLO (full auto, non-interactive)")
	} else {
		fmt.Println("  Mode: Interactive (Ctrl+C to exit)")
	}
	if detectPR {
		fmt.Println("  PR detection: enabled (exits on PR creation)")
	}

	// Create and run the loop
	loopCfg := loop.Config{
		Feature:           feature,
		Project:           project,
		ContainerName:     containerName,
		RepoPath:          repoRoot,
		ExtraEnv:          vibeCfg.GetEnvValues(),
		ExtraMounts:       vibeCfg.GetMounts(),
		Network:           vibeCfg.Network,
		MaxIterations:     maxIterations,
		CompletionPromise: completionPromise,
		PromptFile:        promptFile,
		YoloMode:          yoloMode,
		DetectPR:          detectPR,
	}

	runner := loop.NewRunner(dockerClient, loopCfg)
	return runner.Run()
}

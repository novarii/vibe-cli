package loop

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/novari/vibe-cli/internal/config"
	"github.com/novari/vibe-cli/internal/docker"
)

// Config holds configuration for the loop runner
type Config struct {
	Feature           string
	Project           string
	ContainerName     string            // Resolved container name
	RepoPath          string            // Host repo path to mount at /repo
	ExtraEnv          map[string]string // Additional env vars from .vibe.yaml
	ExtraMounts       map[string]string // Additional volume mounts from .vibe.yaml
	Network           string            // Docker network to connect to
	MaxIterations     int
	CompletionPromise string
	PromptFile        string
	YoloMode          bool // Full auto: uses -p flag, non-interactive
	DetectPR          bool // Exit when PR is created
}

// Runner executes the Claude loop
type Runner struct {
	docker     *docker.Client
	config     Config
	detector   *PromiseDetector
	prDetector *PRDetector
}

// NewRunner creates a new loop runner
func NewRunner(dockerClient *docker.Client, cfg Config) *Runner {
	var prDetector *PRDetector
	if cfg.DetectPR {
		prDetector = NewPRDetector()
	}

	return &Runner{
		docker:     dockerClient,
		config:     cfg,
		detector:   NewPromiseDetector(cfg.CompletionPromise),
		prDetector: prDetector,
	}
}

// Run executes the main loop
func (r *Runner) Run() error {
	// Ensure container is running with env vars and mounts from config
	containerCfg := docker.DefaultContainerConfig(r.config.Project, r.config.RepoPath)
	containerCfg.Name = r.config.ContainerName
	containerCfg.ExtraEnv = r.config.ExtraEnv
	containerCfg.ExtraMounts = r.config.ExtraMounts
	containerCfg.Network = r.config.Network
	if err := r.docker.EnsureContainer(containerCfg); err != nil {
		return err
	}

	// Verify prompt file exists inside container
	wtPath := config.WorktreePath(r.config.Feature)
	promptPath := wtPath + "/" + r.config.PromptFile
	exitCode, _, err := r.docker.ExecNonInteractive(containerCfg.Name, []string{"test", "-f", promptPath})
	if err != nil {
		return fmt.Errorf("failed to check prompt file: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("prompt file not found in container: %s", promptPath)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Run the loop
	iteration := 0
	for {
		iteration++

		// Check max iterations
		if r.config.MaxIterations > 0 && iteration > r.config.MaxIterations {
			fmt.Printf("\nMax iterations (%d) reached. Stopping.\n", r.config.MaxIterations)
			return nil
		}

		// Display iteration info
		fmt.Print("\n" + strings.Repeat("=", 60) + "\n")
		fmt.Printf("ITERATION %d", iteration)
		if r.config.MaxIterations > 0 {
			fmt.Printf(" / %d", r.config.MaxIterations)
		}
		fmt.Print("\n" + strings.Repeat("=", 60) + "\n\n")

		// Run Claude for this iteration
		exitCode, output, err := r.runIteration(containerCfg.Name)
		if err != nil {
			return fmt.Errorf("iteration %d failed: %w", iteration, err)
		}

		// Check for non-zero exit
		// Exit code 130 = killed by Ctrl+C (128 + SIGINT=2)
		if exitCode == 130 || exitCode == 2 {
			fmt.Printf("\nInterrupted (Ctrl+C). Stopping loop.\n")
			return nil
		}
		if exitCode != 0 {
			fmt.Printf("\nClaude exited with code %d. Stopping loop.\n", exitCode)
			return nil
		}

		// Check for completion promise
		if r.detector != nil && r.detector.Detect(output) {
			fmt.Printf("\nCompletion promise <%s> detected. Loop complete!\n", r.config.CompletionPromise)
			return nil
		}

		// Check for PR creation
		if r.prDetector != nil && r.prDetector.Detect(output) {
			prURL := r.prDetector.ExtractURL(output)
			fmt.Printf("\nPR created: %s\nTask complete!\n", prURL)
			return nil
		}

		// Check for interrupt (Ctrl+C kills the whole loop)
		select {
		case <-sigChan:
			fmt.Printf("\nInterrupted. Stopping loop.\n")
			return nil
		default:
			// Continue to next iteration
		}
	}
}

// runIteration runs a single iteration of Claude
func (r *Runner) runIteration(containerName string) (int, string, error) {
	wtPath := config.WorktreePath(r.config.Feature)

	if r.config.YoloMode {
		// YOLO mode: use -p flag with streaming for non-interactive mode
		claudeCmd := []string{
			"sh", "-c",
			fmt.Sprintf("cd %s && claude --dangerously-skip-permissions -p \"$(cat %s)\" --output-format stream-json --verbose", wtPath, r.config.PromptFile),
		}

		// Create stream formatter
		formatter := NewStreamFormatter(os.Stdout)

		// Run with streaming output
		exitCode, output, err := r.docker.ExecStreaming(containerName, claudeCmd, func(line string) {
			formatter.ProcessLine(line)
		})

		return exitCode, output, err
	}

	// Default: interactive mode with piped prompt
	claudeCmd := []string{
		"sh", "-c",
		fmt.Sprintf("cd %s && cat %s | claude --dangerously-skip-permissions", wtPath, r.config.PromptFile),
	}

	// Run with output capture
	exitCode, output, err := r.docker.ExecWithOutput(containerName, claudeCmd)
	return exitCode, output, err
}

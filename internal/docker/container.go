package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/novari/vibe-cli/internal/config"
)

// ContainerConfig holds configuration for creating a container
type ContainerConfig struct {
	Name        string
	Image       string
	WorkDir     string
	WorktreePath string
}

// EnsureContainer ensures a container exists and is running
func (c *Client) EnsureContainer(cfg ContainerConfig) error {
	// Check if container already exists
	if c.ContainerExists(cfg.Name) {
		// Start it if not running
		if !c.ContainerRunning(cfg.Name) {
			fmt.Printf("Starting existing container %s...\n", cfg.Name)
			return c.StartContainer(cfg.Name)
		}
		fmt.Printf("Container %s is already running\n", cfg.Name)
		return nil
	}

	// Check if image exists, pull if not
	if !c.ImageExists(cfg.Image) {
		fmt.Printf("Pulling image %s...\n", cfg.Image)
		if err := c.PullImage(cfg.Image); err != nil {
			return err
		}
	}

	// Prepare volume mounts
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	ghConfigDir := filepath.Join(homeDir, ".config", "gh")

	// Ensure .claude directory exists
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	volumes := []string{
		fmt.Sprintf("%s:/workspace", cfg.WorktreePath),
		fmt.Sprintf("%s:/home/user/.claude", claudeDir),
	}

	// Environment variables for container
	envVars := []string{}

	// Pass GH_TOKEN if set on host (for GitHub CLI auth via PAT)
	if ghToken := os.Getenv("GH_TOKEN"); ghToken != "" {
		envVars = append(envVars, "GH_TOKEN="+ghToken)
	} else if _, err := os.Stat(ghConfigDir); err == nil {
		// Fallback: mount gh config if it exists
		volumes = append(volumes, fmt.Sprintf("%s:/home/user/.config/gh", ghConfigDir))
		envVars = append(envVars, "GH_CONFIG_DIR=/home/user/.config/gh")
	}

	fmt.Printf("Creating container %s...\n", cfg.Name)
	if err := c.CreateContainer(cfg.Name, cfg.Image, cfg.WorkDir, volumes, envVars); err != nil {
		return err
	}

	fmt.Printf("Starting container %s...\n", cfg.Name)
	return c.StartContainer(cfg.Name)
}

// DefaultContainerConfig returns a default container configuration
func DefaultContainerConfig(project, feature, worktreePath string) ContainerConfig {
	return ContainerConfig{
		Name:         config.ContainerName(project, feature),
		Image:        config.DefaultImage,
		WorkDir:      "/workspace",
		WorktreePath: worktreePath,
	}
}

// CleanupContainer stops and removes a container
func (c *Client) CleanupContainer(name string) error {
	if !c.ContainerExists(name) {
		fmt.Printf("Container %s does not exist\n", name)
		return nil
	}

	if c.ContainerRunning(name) {
		fmt.Printf("Stopping container %s...\n", name)
		if err := c.StopContainer(name); err != nil {
			return err
		}
	}

	fmt.Printf("Removing container %s...\n", name)
	return c.RemoveContainer(name)
}

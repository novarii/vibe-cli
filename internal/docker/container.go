package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/novari/vibe-cli/internal/config"
)

// ContainerConfig holds configuration for creating a container
type ContainerConfig struct {
	Name        string
	Image       string
	WorkDir     string
	RepoPath    string            // Host repo path to mount at /repo
	ExtraEnv    map[string]string // Additional env vars from .vibe.yaml
	ExtraMounts map[string]string // Additional volume mounts (source -> target)
	Network     string            // Docker network to connect to
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
	claudeJson := filepath.Join(homeDir, ".claude.json")
	ghConfigDir := filepath.Join(homeDir, ".config", "gh")

	// Ensure .claude directory exists
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	volumes := []string{
		fmt.Sprintf("%s:%s", cfg.RepoPath, config.DefaultRepoMount),
		// Mount at /home/agent/.claude for Claude to find config
		fmt.Sprintf("%s:/home/agent/.claude", claudeDir),
		// Also mount at same absolute path so plugin paths resolve correctly
		fmt.Sprintf("%s:%s", claudeDir, claudeDir),
	}

	// Mount SSH keys for git push (read-only)
	sshDir := filepath.Join(homeDir, ".ssh")
	if _, err := os.Stat(sshDir); err == nil {
		volumes = append(volumes, fmt.Sprintf("%s:/home/agent/.ssh:ro", sshDir))
	}

	// SSH agent socket forwarding for git push/PR creation
	if runtime.GOOS == "darwin" {
		// macOS (Docker Desktop) exposes the host agent at a well-known path
		volumes = append(volumes, "/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock")
	} else {
		// Linux: forward the actual SSH_AUTH_SOCK
		if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
			volumes = append(volumes, fmt.Sprintf("%s:%s", sshAuthSock, sshAuthSock))
		}
	}

	// Add extra mounts from .vibe.yaml
	for source, target := range cfg.ExtraMounts {
		if _, err := os.Stat(source); err == nil {
			volumes = append(volumes, fmt.Sprintf("%s:%s", source, target))
		}
	}

	// Environment variables for container
	envVars := []string{}

	// SSH agent socket env var
	if runtime.GOOS == "darwin" {
		envVars = append(envVars, "SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock")
	} else if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
		envVars = append(envVars, "SSH_AUTH_SOCK="+sshAuthSock)
	}

	// Pass GH_TOKEN if set on host (for GitHub CLI auth via PAT)
	if ghToken := os.Getenv("GH_TOKEN"); ghToken != "" {
		envVars = append(envVars, "GH_TOKEN="+ghToken)
	} else if _, err := os.Stat(ghConfigDir); err == nil {
		// Fallback: mount gh config if it exists
		volumes = append(volumes, fmt.Sprintf("%s:/home/agent/.config/gh", ghConfigDir))
		envVars = append(envVars, "GH_CONFIG_DIR=/home/agent/.config/gh")
	}

	// Add extra env vars from .vibe.yaml
	for name, value := range cfg.ExtraEnv {
		envVars = append(envVars, name+"="+value)
	}

	fmt.Printf("Creating container %s...\n", cfg.Name)
	if err := c.CreateContainer(cfg.Name, cfg.Image, cfg.WorkDir, volumes, envVars, cfg.Network); err != nil {
		return err
	}

	fmt.Printf("Starting container %s...\n", cfg.Name)
	if err := c.StartContainer(cfg.Name); err != nil {
		return err
	}

	// Copy ~/.claude.json into container (not mounted to avoid corruption)
	if _, err := os.Stat(claudeJson); err == nil {
		fmt.Printf("Copying Claude config to container...\n")
		if err := c.CopyFileToContainer(cfg.Name, claudeJson, "/home/agent/.claude.json"); err != nil {
			// Non-fatal, just warn
			fmt.Printf("Warning: failed to copy .claude.json: %v\n", err)
		}
	}

	return nil
}

// DefaultContainerConfig returns a default container configuration
func DefaultContainerConfig(project, repoPath string) ContainerConfig {
	return ContainerConfig{
		Name:     config.ContainerName(project),
		Image:    config.DefaultImage,
		WorkDir:  config.DefaultRepoMount,
		RepoPath: repoPath,
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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DockerExecer is the interface for running commands inside a container.
// This avoids a circular import between config and docker packages.
type DockerExecer interface {
	ExecNonInteractive(containerName string, cmd []string) (int, string, error)
}

// Mount represents a volume mount configuration
type Mount struct {
	Source string `yaml:"source"` // Host path
	Target string `yaml:"target"` // Container path
}

// VibeConfig represents the .vibe.yaml configuration
type VibeConfig struct {
	// Copy lists files/patterns to copy from main repo to worktree
	Copy []string `yaml:"copy"`

	// Env lists environment variables to pass to the container
	Env []string `yaml:"env"`

	// Mounts lists additional volume mounts for the container
	Mounts []Mount `yaml:"mounts"`

	// Network is the Docker network to connect the container to
	Network string `yaml:"network"`

	// PostCreate is a script to run after worktree creation
	PostCreate string `yaml:"post_create"`
}

// LoadVibeConfig loads .vibe.yaml from the given directory
// Returns empty config if file doesn't exist
func LoadVibeConfig(dir string) (*VibeConfig, error) {
	configPath := filepath.Join(dir, ".vibe.yaml")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Try .vibe.yml as alternative
		configPath = filepath.Join(dir, ".vibe.yml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// No config file, return defaults
			return &VibeConfig{}, nil
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg VibeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GetEnvValues returns a map of env var names to values
// Only includes vars that are actually set in the environment
func (c *VibeConfig) GetEnvValues() map[string]string {
	result := make(map[string]string)
	for _, name := range c.Env {
		if val, ok := os.LookupEnv(name); ok {
			result[name] = val
		}
	}
	return result
}

// GetMounts returns a map of source -> target for volume mounts
func (c *VibeConfig) GetMounts() map[string]string {
	result := make(map[string]string)
	for _, m := range c.Mounts {
		result[m.Source] = m.Target
	}
	return result
}

// CopyFilesInContainer copies files matching Copy patterns from /repo to a worktree inside the container
func (c *VibeConfig) CopyFilesInContainer(docker DockerExecer, containerName, feature string) error {
	worktreePath := WorktreePath(feature)

	for _, pattern := range c.Copy {
		src := DefaultRepoMount + "/" + pattern
		dst := worktreePath + "/"

		// Use shell glob expansion for patterns with wildcards
		if strings.ContainsAny(pattern, "*?[") {
			cmd := []string{"sh", "-c", fmt.Sprintf("cp -r %s %s", src, dst)}
			exitCode, output, err := docker.ExecNonInteractive(containerName, cmd)
			if err != nil {
				return fmt.Errorf("failed to copy %s: %w", pattern, err)
			}
			if exitCode != 0 {
				return fmt.Errorf("failed to copy %s: %s", pattern, output)
			}
		} else {
			// Ensure parent directory exists
			dstDir := worktreePath + "/" + filepath.Dir(pattern)
			if dstDir != worktreePath+"/" && dstDir != worktreePath+"/." {
				exitCode, output, err := docker.ExecNonInteractive(containerName, []string{"mkdir", "-p", dstDir})
				if err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dstDir, err)
				}
				if exitCode != 0 {
					return fmt.Errorf("failed to create directory %s: %s", dstDir, output)
				}
			}

			cmd := []string{"cp", "-r", src, worktreePath + "/" + pattern}
			exitCode, output, err := docker.ExecNonInteractive(containerName, cmd)
			if err != nil {
				return fmt.Errorf("failed to copy %s: %w", pattern, err)
			}
			if exitCode != 0 {
				return fmt.Errorf("failed to copy %s: %s", pattern, output)
			}
		}
	}
	return nil
}

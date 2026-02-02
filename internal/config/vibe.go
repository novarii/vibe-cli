package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

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

// CopyFiles copies files matching the Copy patterns from srcDir to dstDir
func (c *VibeConfig) CopyFiles(srcDir, dstDir string) error {
	for _, pattern := range c.Copy {
		srcPath := filepath.Join(srcDir, pattern)

		// Check if it's a glob pattern
		matches, err := filepath.Glob(srcPath)
		if err != nil {
			return err
		}

		// If no matches and no glob chars, treat as literal path
		if len(matches) == 0 {
			matches = []string{srcPath}
		}

		for _, match := range matches {
			// Get relative path from srcDir
			relPath, err := filepath.Rel(srcDir, match)
			if err != nil {
				continue
			}

			dstPath := filepath.Join(dstDir, relPath)

			// Check if source exists
			info, err := os.Stat(match)
			if os.IsNotExist(err) {
				continue // Skip non-existent files
			}
			if err != nil {
				return err
			}

			if info.IsDir() {
				if err := copyDir(match, dstPath); err != nil {
					return err
				}
			} else {
				if err := copyFile(match, dstPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Read source
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Get source permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Write destination
	return os.WriteFile(dst, data, info.Mode())
}

// copyDir recursively copies a directory, following symlinks
func copyDir(src, dst string) error {
	// Resolve symlinks at the source level
	realSrc, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	return filepath.Walk(realSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from the resolved source
		relPath, err := filepath.Rel(realSrc, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		// Check if this is a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Resolve the symlink target
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}

			targetInfo, err := os.Stat(target)
			if err != nil {
				return err
			}

			if targetInfo.IsDir() {
				// Recursively copy symlinked directory
				return copyDir(path, dstPath)
			}
			// For symlinked files, copy the actual file
			return copyFile(target, dstPath)
		}

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

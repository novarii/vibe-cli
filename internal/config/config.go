package config

const (
	// DefaultImage is the Docker image used for Claude containers
	DefaultImage = "docker/sandbox-templates:claude-code"

	// DefaultPromptFile is the default prompt file name
	DefaultPromptFile = "prompt.md"

	// DefaultWorktreeDir is the relative path for worktrees
	DefaultWorktreeDir = "../worktrees"

	// DefaultMaxIterations is the default max iterations (0 = unlimited)
	DefaultMaxIterations = 0
)

// ContainerName generates a container name from project and feature
func ContainerName(project, feature string) string {
	return "claude-" + project + "-" + feature
}

// BranchName generates a branch name from feature
func BranchName(feature string) string {
	return "feature/" + feature
}

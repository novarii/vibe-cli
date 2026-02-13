package config

const (
	// DefaultImage is the Docker image used for Claude containers
	DefaultImage = "vibe-claude:latest"

	// DefaultPromptFile is the default prompt file name
	DefaultPromptFile = "prompt.md"

	// DefaultRepoMount is the container path where the repo is mounted
	DefaultRepoMount = "/repo"

	// DefaultWorktreeBase is the container path for worktrees
	DefaultWorktreeBase = "/worktrees"

	// DefaultMaxIterations is the default max iterations (0 = unlimited)
	DefaultMaxIterations = 0
)

// ContainerName generates a container name from project
func ContainerName(project string) string {
	return "vibe-" + project
}

// BranchName generates a branch name from feature
func BranchName(feature string) string {
	return "feature/" + feature
}

// WorktreePath returns the container path for a feature worktree
func WorktreePath(feature string) string {
	return DefaultWorktreeBase + "/" + feature
}

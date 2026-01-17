# Vibe CLI

Manage isolated development environments with git worktrees and Docker-sandboxed Claude agents.

## Overview

Vibe automates the workflow of creating isolated feature branches with sandboxed AI agents for autonomous development loops. It combines:

- **Git worktrees** for code isolation between features
- **Docker containers** running Claude Code for safe, autonomous development
- **Loop mode** for running Claude repeatedly until task completion

## Installation

```bash
# Clone the repo
git clone https://github.com/novarii/vibe-cli.git
cd vibe-cli

# Build and install
make install
```

Requires:
- Go 1.21+
- Docker
- Git

## Quick Start

```bash
# Create a new feature worktree and start Claude
vibe new auth

# Open an existing worktree
vibe open auth

# Run autonomous loop until PR is created
vibe loop auth --yolo

# Clean up when done
vibe cleanup auth
```

## Commands

### `vibe new <feature> [base]`

Creates a new git worktree and starts an interactive Claude session.

```bash
vibe new auth           # Based on main
vibe new auth develop   # Based on develop branch
```

- Creates worktree at `../worktrees/<project>/<feature>`
- Creates branch `feature/<feature>`
- Starts Docker container with Claude Code
- Mounts worktree to `/workspace`

### `vibe open <feature>`

Opens an existing worktree in a new Claude session.

```bash
vibe open auth
```

### `vibe loop <feature>`

Runs an autonomous loop where Claude executes a prompt file repeatedly.

```bash
vibe loop auth                           # Interactive mode
vibe loop auth --yolo                    # Full auto mode
vibe loop auth --max-iterations 50       # Limit iterations
vibe loop auth --completion-promise DONE # Custom exit condition
```

**Flags:**
- `--yolo` - Non-interactive mode with formatted streaming output
- `--max-iterations N` - Stop after N iterations (0 = unlimited)
- `--completion-promise TEXT` - Stop when `<promise>TEXT</promise>` detected
- `--detect-pr` - Stop when GitHub PR URL detected (default: true)
- `--prompt-file FILE` - Custom prompt file (default: prompt.md)

**Exit conditions:**
1. Max iterations reached
2. Completion promise detected in output
3. PR created (GitHub URL detected)
4. User interrupt (Ctrl+C)
5. Claude error (non-zero exit)

### `vibe list`

Lists all feature worktrees for the current project.

```bash
vibe list
```

### `vibe cleanup <feature>`

Removes a worktree, its branch, and Docker container.

```bash
vibe cleanup auth
vibe cleanup auth --force  # Force removal even with uncommitted changes
```

## How It Works

1. **Worktree isolation**: Each feature gets its own directory and branch via git worktrees
2. **Docker sandbox**: Claude runs in a container with the worktree mounted at `/workspace`
3. **Prompt-driven**: Place a `prompt.md` in your worktree with instructions for Claude
4. **Loop execution**: In loop mode, Claude reads `prompt.md` each iteration and works autonomously

## Container Setup

The Docker container:
- Uses `docker/sandbox-templates:claude-code` image
- Mounts your worktree to `/workspace`
- Mounts `~/.claude` for Claude authentication
- Passes `GH_TOKEN` env var for GitHub CLI auth (if set)

## Environment Variables

- `GH_TOKEN` - GitHub personal access token for `gh` CLI in containers

## Example Workflow

```bash
# 1. Create a prompt file in your repo
cat > prompt.md << 'EOF'
Study specs/ and implement the next incomplete task.
- Write tests first
- Run tests after changes
- When tests pass, commit and open a PR
EOF

# 2. Start a new feature
vibe new calculator-refactor

# 3. Or run fully autonomous
vibe loop calculator-refactor --yolo --max-iterations 20
```

## License

MIT

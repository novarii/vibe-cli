# Vibe CLI

Manage isolated development environments with git worktrees and Docker-sandboxed Claude agents.

## Overview

Vibe mounts your repo into a single long-lived Docker container and creates git worktrees inside it for agent isolation. Multiple agents run as separate processes in one container, each in its own worktree.

- **Single container** per project — shared filesystem, fast worktree creation
- **Git worktrees** inside the container at `/worktrees/<feature>` for code isolation
- **Loop mode** for running Claude repeatedly (in Ralph Wiggum loops) until task completion
- **`.vibe.yaml`** for project-specific config (file copying, env vars, mounts, networking)

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

# Create multiple worktrees at once
vibe new auth payments api

# Open an existing worktree
vibe open auth

# Run autonomous loop until PR is created
vibe loop auth --yolo

# List all worktrees
vibe list

# Open a shell in a worktree
vibe terminal auth

# Clean up when done
vibe cleanup auth

# Stop the container
vibe stop
```

## Commands

### `vibe new <feature> [feature...]`

Creates git worktrees inside the container and starts a Claude session.

```bash
vibe new auth                  # Create worktree, launch Claude
vibe new auth payments api     # Create multiple worktrees
vibe new auth --base develop   # Base on develop branch
```

- Creates worktree at `/worktrees/<feature>` inside the container
- Creates branch `feature/<feature>`
- With a single feature, launches an interactive Claude session
- With multiple features, prints a summary

**Flags:**
- `--base BRANCH` - Base branch for new worktrees (default: `main`)

### `vibe open <feature>`

Reopens an existing worktree and continues the previous Claude session.

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

Lists all worktrees inside the project's container with their branches.

```bash
vibe list
```

### `vibe terminal <feature>`

Opens an interactive shell in the container at the feature's worktree.

```bash
vibe terminal auth
vibe term auth       # alias
vibe sh auth         # alias
```

### `vibe cleanup <feature> [feature...]`

Removes worktrees and branches inside the container.

```bash
vibe cleanup auth                    # Remove worktree and branch
vibe cleanup auth payments           # Remove multiple features
vibe cleanup auth --force            # Force removal with uncommitted changes
vibe cleanup auth --remove-container # Also stop and remove the container
```

**Flags:**
- `--force, -f` - Force removal even with uncommitted changes
- `--remove-container` - Also stop and remove the Docker container

### `vibe stop`

Stops the project's Docker container. It can be restarted later by any command that needs it.

```bash
vibe stop
```

### Global Flags

- `--container NAME` - Override container name suffix (default: project name). Use this to run multiple containers for different contexts:

```bash
vibe new auth --container backend    # Creates container vibe-backend
vibe new auth --container frontend   # Creates container vibe-frontend
```

## How It Works

1. **Single container**: Your repo is bind-mounted at `/repo` inside a long-lived Docker container
2. **Worktree isolation**: Each feature gets its own worktree at `/worktrees/<feature>` with branch `feature/<feature>`
3. **Prompt-driven**: Place a `prompt.md` in your worktree with instructions for Claude
4. **Loop execution**: In loop mode, Claude reads `prompt.md` each iteration and works autonomously

## Container Setup

The Docker container:
- Uses `vibe-claude:latest` image (built from the included Dockerfile)
- Mounts your repo at `/repo`
- Creates worktrees at `/worktrees/<feature>`
- Mounts `~/.claude` for Claude authentication
- Forwards SSH agent socket for git push/PR creation
- Passes `GH_TOKEN` env var for GitHub CLI auth (if set)

## Configuration

Create a `.vibe.yaml` in your project root:

```yaml
# Files to copy from /repo to worktree after creation (for gitignored files)
copy:
  - .env
  - .env.local
  - node_modules

# Environment variables to pass to the container
env:
  - DATABASE_URL
  - API_KEY

# Additional volume mounts
mounts:
  - source: /path/on/host
    target: /path/in/container

# Docker network to connect to
network: my-network

# Script to run after worktree creation
post_create: npm install
```

### Config Options

| Option | Description |
|--------|-------------|
| `copy` | Files/globs to copy from `/repo` to worktree after creation |
| `env` | Environment variable names to pass through to the container |
| `mounts` | Additional volume mounts (source/target pairs) |
| `network` | Docker network to connect the container to |
| `post_create` | Script to run after worktree creation |

## Environment Variables

- `GH_TOKEN` - GitHub personal access token for `gh` CLI in containers
- Any variables listed in `.vibe.yaml` `env` section

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

# 4. Check on progress
vibe list
vibe terminal calculator-refactor

# 5. Clean up
vibe cleanup calculator-refactor
```

## License

MIT

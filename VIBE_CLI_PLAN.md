# Vibe CLI - Implementation Plan

A Go CLI tool for managing isolated development environments with git worktrees and Docker-sandboxed Claude agents.

## Overview

Vibe automates the workflow of creating isolated feature branches with sandboxed AI agents for autonomous development loops.

## Tech Stack

- **Language**: Go 1.21+
- **CLI Framework**: [Cobra](https://github.com/spf13/cobra)
- **Docker**: Docker SDK for Go
- **Git**: go-git or shell exec

## Project Structure

```
vibe/
├── cmd/
│   └── vibe/
│       └── main.go           # Entry point
├── internal/
│   ├── cli/
│   │   ├── root.go           # Root command
│   │   ├── new.go            # vibe new
│   │   ├── open.go           # vibe open
│   │   ├── loop.go           # vibe loop
│   │   ├── cleanup.go        # vibe cleanup
│   │   └── list.go           # vibe list
│   ├── docker/
│   │   ├── client.go         # Docker client wrapper
│   │   ├── container.go      # Container management
│   │   └── exec.go           # Exec into containers
│   ├── git/
│   │   ├── worktree.go       # Worktree operations
│   │   └── project.go        # Project name detection
│   ├── loop/
│   │   ├── runner.go         # Loop execution logic
│   │   └── detector.go       # Completion promise detection
│   └── config/
│       └── config.go         # Configuration handling
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Commands Specification

### `vibe new <feature> [base]`

Creates a new worktree and starts a fresh Claude session.

```bash
vibe new auth           # Base: main
vibe new auth develop   # Base: develop
```

**Steps:**
1. Detect project name from git remote or directory
2. Create directory `../worktrees/<project>/<feature>`
3. Run `git worktree add -b feature/<feature> <path> <base>`
4. Create Docker container `claude-<project>-<feature>`
5. Start Claude with `--dangerously-skip-permissions`

### `vibe open <feature>`

Reopens an existing worktree and continues the previous Claude session.

```bash
vibe open auth
```

**Steps:**
1. Detect project name
2. Verify worktree exists at `../worktrees/<project>/<feature>`
3. Start or reuse Docker container
4. Run Claude with `--dangerously-skip-permissions -c` (continue)

### `vibe loop <feature> [flags]`

Runs a Ralph-style loop with new sessions each iteration.

```bash
vibe loop auth
vibe loop auth --max-iterations 50
vibe loop auth --max-iterations 50 --completion-promise "DONE"
```

**Flags:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--max-iterations` | int | 0 | Max iterations (0 = unlimited) |
| `--completion-promise` | string | "" | Stop when `<promise>X</promise>` detected |

**Steps:**
1. Detect project name
2. Verify worktree and `prompt.md` exist
3. Loop:
   a. Display iteration info
   b. Start Docker container
   c. Run Claude with prompt from `prompt.md`
   d. Capture output
   e. Check for completion promise in output
   f. Check iteration count
   g. Continue or exit

**Exit Conditions:**
- Max iterations reached
- Completion promise detected: `<promise>DONE</promise>`
- User interrupt (Ctrl+C)
- Claude error (non-zero exit)

### `vibe cleanup <feature> [--force]`

Removes worktree and Docker container.

```bash
vibe cleanup auth
vibe cleanup auth --force
```

**Steps:**
1. Detect project name
2. Stop and remove Docker container `claude-<project>-<feature>`
3. Remove git worktree (with `--force` if flag set)
4. Delete branch `feature/<feature>`

### `vibe list`

Lists all worktrees for current project.

```bash
vibe list
```

**Output:**
```
PROJECT: ansa-mes

FEATURE     BRANCH          CONTAINER         STATUS
auth        feature/auth    claude-ansa-mes-auth   running
payments    feature/payments claude-ansa-mes-payments stopped
```

## Core Components

### Docker Client (`internal/docker/`)

```go
type Client struct {
    cli *client.Client
}

func (c *Client) CreateContainer(name, image, workdir string, volumes []string) error
func (c *Client) StartContainer(name string) error
func (c *Client) ExecInteractive(name string, cmd []string) (int, string, error)
func (c *Client) StopContainer(name string) error
func (c *Client) RemoveContainer(name string) error
func (c *Client) ContainerExists(name string) bool
func (c *Client) ContainerRunning(name string) bool
```

### Git Operations (`internal/git/`)

```go
func GetProjectName() (string, error)           // From remote or dirname
func CreateWorktree(path, branch, base string) error
func RemoveWorktree(path string, force bool) error
func DeleteBranch(branch string, force bool) error
func ListWorktrees() ([]Worktree, error)
func WorktreeExists(path string) bool
```

### Loop Runner (`internal/loop/`)

```go
type LoopConfig struct {
    Feature           string
    MaxIterations     int
    CompletionPromise string
    PromptFile        string
}

type Runner struct {
    docker *docker.Client
    config LoopConfig
}

func (r *Runner) Run() error                    // Main loop
func (r *Runner) runIteration(n int) (string, error)
func (r *Runner) detectPromise(output string) bool
```

### Promise Detection

Scans Claude output for:
```
<promise>DONE</promise>
```

Uses regex: `<promise>(.*?)</promise>`

## Configuration

### Defaults

```go
const (
    DefaultImage       = "docker/sandbox-templates:claude-code"
    DefaultPromptFile  = "prompt.md"
    DefaultWorktreeDir = "../worktrees"
    DefaultMaxIter     = 0  // unlimited
)
```

### Container Naming

```
claude-<project>-<feature>
```

Example: `claude-ansa-mes-auth`

### Volume Mounts

```
- <worktree-path>:/workspace
- ~/.claude:/home/user/.claude
```

## Implementation Phases

### Phase 1: Project Setup
- [ ] Initialize Go module
- [ ] Set up Cobra CLI structure
- [ ] Add root command with version/help

### Phase 2: Git Operations
- [ ] Project name detection
- [ ] Worktree create/remove/list
- [ ] Branch management

### Phase 3: Docker Integration
- [ ] Docker client wrapper
- [ ] Container create/start/stop/remove
- [ ] Interactive exec with TTY

### Phase 4: Basic Commands
- [ ] `vibe new` - create worktree + container + run Claude
- [ ] `vibe open` - reopen with `-c` flag
- [ ] `vibe cleanup` - remove everything
- [ ] `vibe list` - show worktrees

### Phase 5: Loop Command
- [ ] Basic loop execution
- [ ] Max iterations support
- [ ] Completion promise detection
- [ ] Output capture and scanning
- [ ] Graceful Ctrl+C handling

### Phase 6: Polish
- [ ] Error handling and user-friendly messages
- [ ] Progress/status output
- [ ] Makefile for builds
- [ ] Cross-platform builds (darwin, linux)
- [ ] README with usage examples

## Build & Distribution

### Makefile

```makefile
build:
	go build -o bin/vibe cmd/vibe/main.go

install: build
	cp bin/vibe /usr/local/bin/

release:
	GOOS=darwin GOARCH=arm64 go build -o bin/vibe-darwin-arm64 cmd/vibe/main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/vibe-darwin-amd64 cmd/vibe/main.go
	GOOS=linux GOARCH=amd64 go build -o bin/vibe-linux-amd64 cmd/vibe/main.go
```

### Installation

```bash
# From source
git clone https://github.com/user/vibe
cd vibe && make install

# Or download binary
curl -sSL https://github.com/user/vibe/releases/latest/download/vibe-darwin-arm64 \
  -o /usr/local/bin/vibe && chmod +x /usr/local/bin/vibe
```

## Future Considerations (Out of Scope)

- Config file (`.viberc`)
- Multiple prompt files
- Output logging
- Parallel loops
- Plugin system
- Claude API integration (vs CLI)

# Refactor: Single Container + Internal Worktrees

**Spec References:**
- [01-container-architecture.md](../01-container-architecture.md) — Architecture, naming, mount layout, --container flag
- [02-container-git.md](../02-container-git.md) — ContainerGit struct and methods
- [03-docker-config.md](../03-docker-config.md) — Docker exec fix, ContainerConfig, constants, CopyFilesInContainer

## Task Overview

| Task | Description | Dependencies | Effort |
|------|-------------|--------------|--------|
| 1 | Core Infrastructure (demux fix, constants, config cleanup) | None | Medium |
| 2 | Container Config & Flags | Task 1 | Medium |
| 3 | ContainerGit Operations | Tasks 1, 2 | Medium |
| 4 | Feature Commands (new, open, terminal) | Tasks 2, 3 | Medium |
| 5 | Loop System (loop command + runner) | Tasks 2, 3 | Medium |
| 6 | Management Commands & Verification (list, cleanup, stop, build) | Tasks 2, 3 | Medium |

---

## Task 1: Core Infrastructure

**Goal:** Fix the ExecNonInteractive demux bug, update constants/naming, remove dead code from project.go, replace host-side file copy with in-container copy.

**Spec refs:** [03-docker-config.md](../03-docker-config.md) (demux fix, constants, CopyFilesInContainer, GetMainGitDir removal)

### Subtasks
- [ ] 1.1 Fix `ExecNonInteractive` in `internal/docker/client.go`: replace `io.ReadAll(resp.Reader)` with `stdcopy.StdCopy(&stdoutBuf, &stderrBuf, resp.Reader)`, return `stdoutBuf.String()`
- [ ] 1.2 Update `internal/config/config.go`: remove `DefaultWorktreeDir`, add `DefaultRepoMount = "/repo"` and `DefaultWorktreeBase = "/worktrees"`
- [ ] 1.3 Add `WorktreePath(feature string) string` function to `config.go` returning `/worktrees/<feature>`
- [ ] 1.4 Change `ContainerName(project, feature string)` to `ContainerName(project string)` returning `"vibe-" + project`
- [ ] 1.5 Remove `GetMainGitDir()` from `internal/git/project.go`
- [ ] 1.6 Add `CopyFilesInContainer` method to `VibeConfig` in `internal/config/vibe.go` — runs `docker exec cp` commands. Use an interface to avoid circular imports with docker package.
- [ ] 1.7 Remove host-side `CopyFiles`, `copyFile`, `copyDir` from `vibe.go`
- [ ] 1.8 Verify `go build ./...` compiles (expect errors from callers — that's fine, they're updated in later tasks)

### Files Changed
- `internal/docker/client.go` — ExecNonInteractive fix
- `internal/config/config.go` — Constants and naming
- `internal/git/project.go` — Remove GetMainGitDir
- `internal/config/vibe.go` — CopyFilesInContainer, remove host-side copy

### Notes
- The demux fix is critical — all ContainerGit operations depend on clean stdout output
- `stdcopy` is already imported in client.go
- ContainerName signature change will break callers (list.go, cleanup.go, etc.) — those are fixed in tasks 4-6
- CopyFilesInContainer needs to accept an interface (not `*docker.Client`) to avoid circular import between config and docker packages

---

## Task 2: Container Config & Flags

**Goal:** Rewrite ContainerConfig/EnsureContainer for the new mount layout, add `--container` flag to root command.

**Spec refs:** [01-container-architecture.md](../01-container-architecture.md) (mount layout, --container flag), [03-docker-config.md](../03-docker-config.md) (ContainerConfig struct, EnsureContainer)

### Subtasks
- [ ] 2.1 Update `ContainerConfig` struct in `internal/docker/container.go`: remove `WorktreePath` and `MainGitDir`, add `RepoPath string`
- [ ] 2.2 Change `DefaultContainerConfig(project, feature, worktreePath, mainGitDir)` to `DefaultContainerConfig(project, repoPath string)`
- [ ] 2.3 Rewrite `EnsureContainer` volume mounts: `<repoPath>:/repo` replaces worktree mount, remove anonymous `/workspace/node_modules`, remove `MainGitDir` mount hack
- [ ] 2.4 Add SSH agent socket forwarding: macOS uses `/run/host-services/ssh-auth.sock`, Linux uses `$SSH_AUTH_SOCK`. Detect via `runtime.GOOS`. Set `SSH_AUTH_SOCK` env var in container.
- [ ] 2.5 Update `EnsureContainer` WorkDir from `/workspace` to use `cfg.WorkDir` (which defaults to `/repo`)
- [ ] 2.6 Add persistent `--container` string flag to `internal/cli/root.go`
- [ ] 2.7 Add `resolveContainerName(project string) string` helper to root.go — returns `"vibe-" + containerFlag` if set, else `config.ContainerName(project)`
- [ ] 2.8 Register `stopCmd` in root.go init (the command itself is created in Task 6)
- [ ] 2.9 Update root command description to reflect new architecture

### Files Changed
- `internal/docker/container.go` — ContainerConfig, DefaultContainerConfig, EnsureContainer
- `internal/cli/root.go` — --container flag, resolveContainerName, register stopCmd

### Notes
- `CleanupContainer` stays unchanged — it already works by container name
- Keep `.claude.json` copy-to-container logic in EnsureContainer
- The `--container` flag value is accessed by subcommands, so make it a package-level var or use cobra's persistent flag system

---

## Task 3: ContainerGit Operations

**Goal:** Rewrite `internal/git/worktree.go` with a `ContainerGit` struct that runs all git operations inside the container via `docker exec`.

**Spec refs:** [02-container-git.md](../02-container-git.md) (full ContainerGit API)

### Subtasks
- [ ] 3.1 Define `ContainerGit` struct with `Docker *docker.Client` and `ContainerName string`
- [ ] 3.2 Implement `CreateWorktree(feature, base string) error` — runs `git -C /repo worktree add -b feature/<feat> /worktrees/<feat> <base>`
- [ ] 3.3 Implement `RemoveWorktree(feature string, force bool) error` — runs `git -C /repo worktree remove /worktrees/<feat>`, with force fallback to `rm -rf` + `git worktree prune`
- [ ] 3.4 Implement `DeleteBranch(branch string, force bool) error` — runs `git -C /repo branch -d/-D <branch>`
- [ ] 3.5 Implement `ListWorktrees() ([]Worktree, error)` — runs `git -C /repo worktree list --porcelain`, parses output, filters to `/worktrees/` paths
- [ ] 3.6 Implement `WorktreeExists(feature string) (bool, error)` — runs `test -d /worktrees/<feature>`, checks exit code
- [ ] 3.7 Keep `FeatureFromBranch()` as standalone utility function
- [ ] 3.8 Remove all old host-side functions: `GetWorktreePath`, `CreateWorktree`, `RemoveWorktree`, `DeleteBranch`, `ListWorktrees`, `WorktreeExists`, `GetFeatureWorktrees`

### Files Changed
- `internal/git/worktree.go` — Full rewrite

### Notes
- All methods depend on `ExecNonInteractive` returning clean stdout (Task 1)
- `ListWorktrees` parsing is same logic as current, just operating on docker exec output
- The `Worktree` struct is unchanged
- Remove the `os/exec` and `os` imports; add dependency on `docker` package

---

## Task 4: Feature Commands (new, open, terminal)

**Goal:** Rewrite `vibe new`, `vibe open`, and `vibe terminal` for the container-centric architecture.

**Spec refs:** [01-container-architecture.md](../01-container-architecture.md) (working directories, multi-feature), [02-container-git.md](../02-container-git.md) (ContainerGit usage)

### Subtasks — `vibe new` (`internal/cli/new.go`)
- [ ] 4.1 Change args to `cobra.MinimumNArgs(1)` — accept multiple features
- [ ] 4.2 Add `--base` flag (default "main"), remove positional base arg
- [ ] 4.3 Rewrite flow: get project/repoRoot, resolve container name, `EnsureContainer` with repo mounted at `/repo`
- [ ] 4.4 For each feature: `ContainerGit.CreateWorktree()`, then `CopyFilesInContainer` if configured
- [ ] 4.5 If single feature: launch interactive Claude (`cd /worktrees/<feat> && claude --dangerously-skip-permissions`)
- [ ] 4.6 If multiple features: print summary table, don't launch Claude

### Subtasks — `vibe open` (`internal/cli/open.go`)
- [ ] 4.7 Remove host-side worktree/git checks (no more `GetWorktreePath`, `WorktreeExists` on host)
- [ ] 4.8 Ensure container running, verify worktree exists via `ContainerGit.WorktreeExists`
- [ ] 4.9 `ExecInteractive` with `cd /worktrees/<feat> && claude --dangerously-skip-permissions -c` (fallback without `-c`)

### Subtasks — `vibe terminal` (`internal/cli/terminal.go`)
- [ ] 4.10 Change shell command from `bash` to `bash -c "cd /worktrees/<feat> && exec bash"`
- [ ] 4.11 Use `resolveContainerName` instead of `config.ContainerName(project, feature)`

### Files Changed
- `internal/cli/new.go` — Multi-feature, container-centric, --base flag
- `internal/cli/open.go` — Container-centric, remove host checks
- `internal/cli/terminal.go` — cd to worktree path, new container name

---

## Task 5: Loop System

**Goal:** Rewrite the loop command and runner for container-side paths and the new config.

**Spec refs:** [01-container-architecture.md](../01-container-architecture.md) (working directories), [03-docker-config.md](../03-docker-config.md) (ContainerConfig)

### Subtasks — `internal/cli/loop.go`
- [ ] 5.1 Remove `WorktreePath` and `MainGitDir` from loop config construction
- [ ] 5.2 Add `ContainerName` and `RepoPath` to loop config construction
- [ ] 5.3 Use `resolveContainerName` for container name
- [ ] 5.4 Remove host-side worktree existence check (`git.WorktreeExists`)

### Subtasks — `internal/loop/runner.go`
- [ ] 5.5 Update `Config` struct: remove `WorktreePath`/`MainGitDir`, add `ContainerName`/`RepoPath`
- [ ] 5.6 Change prompt file check from `os.Stat` on host to `docker exec test -f /worktrees/<feat>/<prompt>`
- [ ] 5.7 Update `EnsureContainer` call to use `DefaultContainerConfig(project, repoPath)` with Name override to `ContainerName`
- [ ] 5.8 Update YOLO command: `cd /worktrees/<feat> && claude -p "$(cat <prompt>)" --output-format stream-json --verbose`
- [ ] 5.9 Update interactive command: `cd /worktrees/<feat> && cat <prompt> | claude --dangerously-skip-permissions`

### Files Changed
- `internal/cli/loop.go` — New config fields
- `internal/loop/runner.go` — Container-side paths, container-side prompt check

### Notes
- `runner.go` currently checks prompt file on host (`os.Stat`). New version checks inside container via `docker exec test -f`.
- Container name comes from the CLI (resolved via `resolveContainerName`), not computed inside runner
- `stream.go` and `detector.go` are unchanged

---

## Task 6: Management Commands & Verification

**Goal:** Rewrite `vibe list`, `vibe cleanup`, create `vibe stop`, remove dead code, verify build.

**Spec refs:** [01-container-architecture.md](../01-container-architecture.md) (container lifecycle, --remove-container), [02-container-git.md](../02-container-git.md) (ListWorktrees, RemoveWorktree)

### Subtasks — `vibe list` (`internal/cli/list.go`)
- [ ] 6.1 Check if container exists/running using resolved container name
- [ ] 6.2 Use `ContainerGit.ListWorktrees()` to list worktrees inside container
- [ ] 6.3 Display: container name, container status, table of features/branches

### Subtasks — `vibe cleanup` (`internal/cli/cleanup.go`)
- [ ] 6.4 Change args to `cobra.MinimumNArgs(1)` — accept multiple features
- [ ] 6.5 Add `--remove-container` flag
- [ ] 6.6 For each feature: `ContainerGit.RemoveWorktree()`, `ContainerGit.DeleteBranch()`
- [ ] 6.7 If `--remove-container`: `CleanupContainer()` then `git worktree prune` on host

### Subtasks — `vibe stop` (`internal/cli/stop.go` — **new file**)
- [ ] 6.8 Create `stopCmd` with `cobra.NoArgs`
- [ ] 6.9 Resolve container name, call `dockerClient.StopContainer()`
- [ ] 6.10 Print status message

### Subtasks — Verification
- [ ] 6.11 Remove any remaining dead code (old host-side worktree references, unused imports)
- [ ] 6.12 Run `go build ./...` — compiles cleanly
- [ ] 6.13 Run `go vet ./...` — no issues

### Files Changed
- `internal/cli/list.go` — Container-centric worktree listing
- `internal/cli/cleanup.go` — Multi-feature, --remove-container
- `internal/cli/stop.go` — **New file**
- Various files — dead code removal

---

## Execution Order

```
Task 1 (Core Infrastructure)
    |
Task 2 (Container Config & Flags)
    |
Task 3 (ContainerGit Operations)
    |
    +-- Task 4 (Feature Commands) --------+
    |                                      |
    +-- Task 5 (Loop System) -------------+  (parallel possible)
    |                                      |
    +-- Task 6 (Management & Verify) -----+
```

Tasks 4, 5, and 6 can run in parallel after Task 3 completes.

---

## Files Changed Summary

| File | Task | Change |
|------|------|--------|
| `internal/docker/client.go` | 1 | Fix ExecNonInteractive demuxing |
| `internal/config/config.go` | 1 | New constants, change ContainerName signature |
| `internal/git/project.go` | 1 | Remove GetMainGitDir() |
| `internal/config/vibe.go` | 1 | CopyFilesInContainer, remove host-side copy |
| `internal/docker/container.go` | 2 | New ContainerConfig, new mount layout |
| `internal/cli/root.go` | 2 | --container flag, resolveContainerName, register stop |
| `internal/git/worktree.go` | 3 | Full rewrite — ContainerGit struct |
| `internal/cli/new.go` | 4 | Multi-feature, container-centric, --base flag |
| `internal/cli/open.go` | 4 | Container-centric, remove host checks |
| `internal/cli/terminal.go` | 4 | cd to worktree path |
| `internal/cli/loop.go` | 5 | New config fields |
| `internal/loop/runner.go` | 5 | Container-side paths, prompt check |
| `internal/cli/list.go` | 6 | List worktrees from container |
| `internal/cli/cleanup.go` | 6 | Multi-feature, --remove-container flag |
| `internal/cli/stop.go` | 6 | **New** — vibe stop command |

**No changes:** `internal/docker/exec.go`, `internal/loop/detector.go`, `internal/loop/stream.go`, `Dockerfile`, `cmd/vibe/main.go`

---

## Definition of Done

This refactor is complete when:
- [ ] All 6 tasks marked complete
- [ ] `go build ./...` compiles cleanly
- [ ] `go vet ./...` passes
- [ ] `vibe new auth` creates container + worktree inside, launches Claude
- [ ] `vibe new auth payments` creates both worktrees, no Claude launched
- [ ] `vibe list` shows container + worktrees from inside container
- [ ] `vibe loop auth --yolo --max-iterations 1` runs one iteration
- [ ] `vibe terminal auth` opens shell at `/worktrees/auth`
- [ ] `vibe cleanup auth` removes worktree, container persists
- [ ] `vibe stop` stops the container
- [ ] `vibe new api --container backend` creates separate container

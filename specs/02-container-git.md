# Spec: ContainerGit Operations

## Problem

All git operations currently run on the host via `exec.Command("git", ...)`. In the new architecture, worktrees live inside the container. Git operations must run inside the container via `docker exec`.

## Design

Replace host-side git functions with a `ContainerGit` struct that runs git commands via `docker.ExecNonInteractive`.

```go
type ContainerGit struct {
    Docker        *docker.Client
    ContainerName string
}
```

**Dependency:** `ExecNonInteractive` must correctly demux stdout/stderr (see spec 03-docker-config.md) for all git output parsing to work.

## Methods

### `CreateWorktree(feature, base string) error`

```
git -C /repo worktree add -b feature/<feature> /worktrees/<feature> <base>
```

- Uses `config.BranchName(feature)` for branch name
- Uses `config.WorktreePath(feature)` for worktree path
- Returns error if non-zero exit code

### `RemoveWorktree(feature string, force bool) error`

```
git -C /repo worktree remove /worktrees/<feature> [--force]
```

- If force removal fails, fall back to: `rm -rf /worktrees/<feature>` + `git -C /repo worktree prune`
- Uses `config.WorktreePath(feature)` for path

### `DeleteBranch(branch string, force bool) error`

```
git -C /repo branch -d/-D <branch>
```

- `-d` for normal delete, `-D` for force delete

### `ListWorktrees() ([]Worktree, error)`

```
git -C /repo worktree list --porcelain
```

- Parse output same as current `ListWorktrees()` (porcelain format)
- Filter results to only paths under `/worktrees/` (exclude `/repo` itself)
- Return `[]Worktree` with Path, Branch, Head fields

### `WorktreeExists(feature string) (bool, error)`

```
test -d /worktrees/<feature>
```

- Check exit code: 0 = exists, non-zero = doesn't exist
- Returns `(bool, error)` — error only for docker exec failures, not for "doesn't exist"

## Kept as Standalone

`FeatureFromBranch(branch string) string` stays as a standalone utility function — it's pure string manipulation with no git operations.

## Removed Functions (from host-side git)

| Function | Replacement |
|----------|------------|
| `GetWorktreePath()` | `config.WorktreePath()` |
| `CreateWorktree()` | `ContainerGit.CreateWorktree()` |
| `RemoveWorktree()` | `ContainerGit.RemoveWorktree()` |
| `DeleteBranch()` | `ContainerGit.DeleteBranch()` |
| `ListWorktrees()` | `ContainerGit.ListWorktrees()` |
| `WorktreeExists()` | `ContainerGit.WorktreeExists()` |
| `GetFeatureWorktrees()` | `ContainerGit.ListWorktrees()` (callers filter) |
| `GetMainGitDir()` | Removed entirely |

## Worktree Struct

No changes — still:

```go
type Worktree struct {
    Path   string
    Branch string
    Head   string
}
```

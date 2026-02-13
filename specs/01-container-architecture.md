# Spec: Single Container + Internal Worktrees Architecture

## Problem

Currently each feature gets its own host-side git worktree AND its own Docker container (`claude-<project>-<feature>`). This is "double isolation" — Docker already provides process/filesystem isolation, and worktrees on the host are unnecessary overhead.

## Architecture

Mount the host repo into a single long-lived Docker container. Create git worktrees *inside* the container for agent isolation. Multiple agents run as separate processes in one container, each in its own worktree.

```
Host: project repo (mounted read-write at /repo)
Container "vibe-<project>":
    /repo/              <- mounted host repo
    /worktrees/auth/    <- agent 1
    /worktrees/payments/<- agent 2
    /worktrees/ui/      <- agent 3
```

## Naming Convention

| Old | New |
|-----|-----|
| `claude-<project>-<feature>` | `vibe-<project>` |
| One container per feature | One container per project |

- `config.ContainerName(project, feature string)` becomes `config.ContainerName(project string)` returning `"vibe-" + project`
- Multiple features share the same container

## Mount Layout

| Host Path | Container Path | Purpose |
|-----------|---------------|---------|
| `<repoPath>` | `/repo` | Host repo (read-write) |
| `~/.claude` | `/home/agent/.claude` | Claude config |
| `~/.ssh` | `/home/agent/.ssh:ro` | SSH keys (read-only) |
| `~/.config/gh` | `/home/agent/.config/gh` | GitHub CLI (fallback) |
| Extra mounts from `.vibe.yaml` | Per config | User-defined |

### Removed mounts (vs current)
- **`/workspace`** — replaced by `/repo` (the whole repo, not a single worktree)
- **Anonymous `/workspace/node_modules`** — unnecessary since worktrees are created inside Linux container (no platform mismatch)
- **`MainGitDir` at same absolute path** — unnecessary since git operations happen inside container where `/repo/.git` is directly accessible

## Container Lifecycle

- Container is **persistent** — survives across `vibe new`, `vibe open`, `vibe loop` invocations
- `vibe stop` stops the container (preserves worktree data)
- `vibe cleanup <feature> --remove-container` removes the container entirely
- Container is NOT removed when individual features are cleaned up (it's shared)

## `--container` Flag

A persistent `--container` flag on the root command enables multiple containers for different contexts:

```bash
vibe new auth                          # uses default: vibe-<project>
vibe new api --container backend       # uses: vibe-backend
vibe list --container backend          # lists worktrees in vibe-backend
```

Resolution logic: `resolveContainerName(project string) string`
- If `--container` is set: return `"vibe-" + containerFlag`
- Otherwise: return `config.ContainerName(project)` (i.e., `"vibe-" + project`)

## Constants

| Old | New |
|-----|-----|
| `DefaultWorktreeDir = "../worktrees"` | Removed |
| — | `DefaultRepoMount = "/repo"` |
| — | `DefaultWorktreeBase = "/worktrees"` |

New helper: `WorktreePath(feature string) string` returns `/worktrees/<feature>`

## Working Directories

| Command | Working directory inside container |
|---------|----------------------------------|
| `vibe new` / Claude launch | `/worktrees/<feature>` |
| `vibe open` | `/worktrees/<feature>` |
| `vibe loop` (YOLO) | `/worktrees/<feature>` |
| `vibe loop` (interactive) | `/worktrees/<feature>` |
| `vibe terminal` | `/worktrees/<feature>` |

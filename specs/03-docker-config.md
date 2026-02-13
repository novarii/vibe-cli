# Spec: Docker & Config Changes

## ExecNonInteractive Demux Fix

**File:** `internal/docker/client.go` (lines 200-232)

### Problem

Currently reads raw bytes with `io.ReadAll(resp.Reader)`. Without TTY, Docker multiplexes stdout/stderr with 8-byte headers. Must use `stdcopy.StdCopy` to demux. All subsequent git-via-docker-exec operations depend on clean output.

### Solution

Replace:
```go
output, err := io.ReadAll(resp.Reader)
```

With:
```go
var stdoutBuf, stderrBuf bytes.Buffer
_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, resp.Reader)
```

Return `stdoutBuf.String()` as the output. Note: `stdcopy` is already imported in client.go (used by `PullImage`).

## ContainerConfig Changes

**File:** `internal/docker/container.go`

### Old struct
```go
type ContainerConfig struct {
    Name         string
    Image        string
    WorkDir      string
    WorktreePath string
    MainGitDir   string
    ExtraEnv     map[string]string
    ExtraMounts  map[string]string
    Network      string
}
```

### New struct
```go
type ContainerConfig struct {
    Name        string
    Image       string
    WorkDir     string
    RepoPath    string              // Host repo path to mount at /repo
    ExtraEnv    map[string]string
    ExtraMounts map[string]string
    Network     string
}
```

- `WorktreePath` removed (worktrees are created inside container)
- `MainGitDir` removed (no longer needed — /repo/.git is directly accessible)
- `RepoPath` added (host repo root to mount)
- `WorkDir` changes from `/workspace` to `/repo`

### DefaultContainerConfig

Old: `DefaultContainerConfig(project, feature, worktreePath, mainGitDir string)`
New: `DefaultContainerConfig(project, repoPath string)`

```go
func DefaultContainerConfig(project, repoPath string) ContainerConfig {
    return ContainerConfig{
        Name:     config.ContainerName(project),
        Image:    config.DefaultImage,
        WorkDir:  config.DefaultRepoMount,
        RepoPath: repoPath,
    }
}
```

### EnsureContainer Mount Layout

New volumes list:
```go
volumes := []string{
    fmt.Sprintf("%s:/repo", cfg.RepoPath),
    fmt.Sprintf("%s:/home/agent/.claude", claudeDir),
    fmt.Sprintf("%s:%s", claudeDir, claudeDir),
}
```

Removed:
- `/workspace/node_modules` anonymous volume
- `MainGitDir` mount hack

Kept:
- `.claude` directory mounts
- `.ssh` read-only mount
- GH config / GH_TOKEN
- Extra mounts from `.vibe.yaml`
- `.claude.json` copy
- Network configuration

### SSH Agent Socket Forwarding (new)

Currently SSH keys are mounted read-only but the SSH agent socket is **not** forwarded. This means passphrase-protected keys can't be used, and agents can't create PRs.

**Fix:** Forward the host's SSH agent socket into the container.

On macOS (Docker Desktop), Docker exposes the host agent at a well-known path:
```go
// SSH agent forwarding for git push/PR creation
volumes = append(volumes, "/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock")
envVars = append(envVars, "SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock")
```

On Linux, forward the actual `SSH_AUTH_SOCK`:
```go
if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
    volumes = append(volumes, fmt.Sprintf("%s:%s", sshAuthSock, sshAuthSock))
    envVars = append(envVars, "SSH_AUTH_SOCK="+sshAuthSock)
}
```

**Implementation:** Detect platform (`runtime.GOOS`) and use the appropriate mechanism. This is secure — private keys never leave the host's SSH agent.

## CopyFilesInContainer

**File:** `internal/config/vibe.go`

### New function

```go
func (c *VibeConfig) CopyFilesInContainer(dockerClient *docker.Client, containerName, feature string) error
```

For each pattern in `Copy`:
- Run `docker exec cp -r /repo/<pattern> /worktrees/<feature>/<pattern>` (or equivalent)
- Handle globs: `docker exec sh -c "cp -r /repo/<glob> /worktrees/<feature>/"`

### Removed

- `CopyFiles(srcDir, dstDir string) error` — host-side copy method
- `copyFile(src, dst string) error` — helper
- `copyDir(src, dst string) error` — helper

### Note on imports

`CopyFilesInContainer` takes a `*docker.Client`, which means `config` package imports `docker` package. If this creates a circular dependency, the function should live in a different package (e.g., as a standalone function in `docker` package or a new `util` package). Alternatively, accept an interface:

```go
type DockerExecer interface {
    ExecNonInteractive(containerName string, cmd []string) (int, string, error)
}
```

## Constants Changes

**File:** `internal/config/config.go`

| Change | Details |
|--------|---------|
| Remove | `DefaultWorktreeDir = "../worktrees"` |
| Add | `DefaultRepoMount = "/repo"` |
| Add | `DefaultWorktreeBase = "/worktrees"` |
| Add | `WorktreePath(feature string) string` returning `/worktrees/<feature>` |
| Change | `ContainerName(project, feature string)` -> `ContainerName(project string)` returning `"vibe-" + project` |
| Keep | `BranchName(feature string) string` — unchanged |
| Keep | `DefaultImage`, `DefaultPromptFile`, `DefaultMaxIterations` — unchanged |

## Removed from project.go

`GetMainGitDir()` — no longer needed since the entire repo is mounted at `/repo` and git operations happen inside the container.

Kept:
- `GetProjectName()` — still needed on host to derive container name
- `GetRepoRoot()` — still needed on host to know what to mount

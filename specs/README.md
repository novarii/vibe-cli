# Specs Directory

This directory contains specification documents for vibe-cli, following spec-driven development principles.

## What is a Spec?

A **spec** is an atomic source of truth document. It can contain:
- Requirements and constraints
- Architecture decisions and rationale
- Code patterns and guidelines
- Implementation standards

**Key principles:**
- 1 topic of concern = 1 spec file
- Specs are referenced by implementation tasks
- Implementation plans should be self-contained (reference specs or include all needed info)

---

## Spec Lookup Table

*Last updated: 2026-02-12*

| Spec | Description | Key Topics |
|------|-------------|------------|
| [01-container-architecture.md](./01-container-architecture.md) | Single container + internal worktrees architecture | Mount layout, naming (`vibe-<project>`), `--container` flag, container lifecycle |
| [02-container-git.md](./02-container-git.md) | ContainerGit operations | `ContainerGit` struct, docker exec git commands, worktree CRUD inside container |
| [03-docker-config.md](./03-docker-config.md) | Docker & config changes | ExecNonInteractive demux fix, ContainerConfig rewrite, SSH agent forwarding, CopyFilesInContainer, constants |

## Tasks

| Task File | Description | Status |
|-----------|-------------|--------|
| [tasks/refactor-internal-worktrees.md](./tasks/refactor-internal-worktrees.md) | Refactor to single container + internal worktrees | Pending |

---

## Adding New Specs

When adding a new spec:

1. Identify the **topic of concern** (one topic per spec)
2. Create `specs/{topic-name}.md`
3. Add to the lookup table above
4. Link from related specs if needed

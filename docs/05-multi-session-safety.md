# 5. Multi-session Safety

Source: `packages/agent/src/harness/session/jsonl-storage.ts`, `packages/agent/src/harness/session/jsonl-repo.ts`, `AGENTS.md`.

`AGENTS.md` says:

> Multiple pi sessions may be running in this cwd at the same time, each modifying different files.

**Surprising finding**: there is **no filesystem locking, no daemon, no inter-process coordination**. Multi-session safety is achieved entirely through **architecture + convention**.

## How pi achieves it

### 1. One JSONL file per session, one writer per file

Different concurrent pi runs in the same cwd land in different files:

```
<sessionsRoot>/
  --<encoded-cwd>--/
    2026-05-18T10-30-45-123Z_<uuid>.jsonl
    2026-05-18T11-15-02-456Z_<uuid>.jsonl     ← session B
    2026-05-18T11-15-30-789Z_<uuid>.jsonl     ← session C started 28s after B
```

Filename = `<timestamp-with-millis>_<sessionId>.jsonl`. Two sessions launched even within the same millisecond would collide on timestamp but their UUIDs differ, so filenames remain distinct.

**No two sessions ever write the same file.**

### 2. Append-only writes via `appendFile`

The harness never seeks or rewrites. All mutations are `fs.appendFile` calls.

Single-writer append is **atomic on POSIX up to PIPE_BUF** (4096 bytes on Linux). Pi's entries are small JSON lines that comfortably fit; even for larger entries, since there is only one writer per file, there's no interleaving to worry about.

### 3. UUIDv7 + per-cwd directory partitioning

`uuid.ts` generates time-ordered UUIDv7 with per-millisecond sequence counter (collision-resistant even within the same ms). Combined with cwd-encoded directory partitioning, filenames cannot collide.

### 4. Cooperative git rules in AGENTS.md

This is the **convention** half of the design — rules for the agent, not code.

From `AGENTS.md`:

> Git operations that touch unstaged, staged, or untracked files outside your own changes will stomp on other sessions' work. Follow these rules:
>
> **Committing:**
> - Only commit files YOU changed in THIS session.
> - Stage explicit paths (`git add <path1> <path2>`); never `git add -A` / `git add .`.
> - Before committing, run `git status` and verify you are only staging your files.
>
> **Never run** (destroys other agents' work or bypasses checks):
> - `git reset --hard`, `git checkout .`, `git clean -fd`, `git stash`,
>   `git add -A`, `git add .`, `git commit --no-verify`.
>
> **If rebase conflicts occur:**
> - Resolve conflicts only in files you modified.
> - If a conflict is in a file you did not modify, abort and ask the user.
> - Never force push.

A pre-commit hook also blocks lockfile commits unless `PI_ALLOW_LOCKFILE_CHANGE=1`.

### 5. Session cwd is recorded in the header

The session header includes `cwd: string`. Opening a session validates which cwd it belongs to, so you can't accidentally mix sessions across projects.

## Implications for the Go port

### Use the same model

- Per-session append-only JSONL file.
- UUIDv7-keyed.
- Cwd-partitioned directory structure.
- Append via `os.OpenFile(path, O_APPEND|O_WRONLY, 0644)` + `Write`.

### Do NOT reach for `flock`

`flock` is a footgun:
- Broken or unreliable on NFS.
- Inconsistent on Windows.
- Doesn't protect against the actual problem (which is shared-resource conflicts like git, not file conflicts).

The file-per-session design eliminates the need.

### Encode the git rules INSIDE the tools, not in the system prompt

This is where pi's design has a soft spot — the rules are in `AGENTS.md` (rules for the LLM), and rely on the LLM to obey. Two LLMs running in parallel may both decide to do `git stash` to "be safe" and stomp on each other.

**In Go, encode these rules in the tools themselves:**

```go
// in the bash tool
func (b *BashTool) Execute(ctx context.Context, ...) (ToolResult, error) {
    if isBannedGitCommand(args.Command) {
        return ToolResult{
            IsError: true,
            Content: []ContentBlock{Text("This command is banned in multi-session mode: <reason>")},
        }, nil
    }
    // ...
}

func isBannedGitCommand(cmd string) bool {
    // pattern match: git add -A, git add ., git stash, git reset --hard,
    // git checkout ., git clean -fd, git commit --no-verify, etc.
}
```

Or provide a dedicated `git` tool that only allows whitelisted subcommands and explicit-path-only operations.

### Lock-free ≠ conflict-free for shared resources

The append-only file model is lock-free for the **session log**. It does NOT protect against:
- Two sessions writing to the same source file simultaneously.
- Two sessions running git operations that conflict.
- Two sessions running long-running shell commands that compete for CPU/disk.

These remain the **tool author's responsibility** (or the user's choice — they accepted this when they ran two sessions in one cwd).

## What's elegant

- **No locks = no deadlocks, no NFS edge cases, no Windows quirks.**
- **The session log is the source of truth.** Never corrupted by concurrent writers because there is only one writer.
- **Forking is just `cp`-with-truncation.** New session, new file, new UUID, parent path recorded in header.
- **Cwd partitioning makes per-project listing trivial.** No global index to maintain.

## Caveats

- **Multi-session "safety" relies on convention** for git. AGENTS.md is rules-for-the-agent, not code. **Make this code in the Go port.**
- **The harness assumes one writer per session file.** If you ever spawn child harnesses sharing a session, you need either an explicit handoff or per-harness session files.
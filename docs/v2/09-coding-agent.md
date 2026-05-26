# 09 — Coding-Agent Product Layer

> **Scope**: coding-agent product layer on tau core — tools, skills, prompts, CLI.
> **Basis**: `01-core-design.md`, `05-access-points.md`, `03-tool-schema.md`.
> **Status**: Design phase. Zero core changes needed.

---

## §1 — Architecture

```
cmd/tau/              CLI entry (single binary: tau)
pkg/coding/           Product layer (tools, skills, prompts, sandbox)
core/                 Unchanged — Loop, Session, ToolRegistry, HookSet
```

Product layer lives entirely outside `core/`. All sandboxing lives inside `Tool.Execute` closures.

---

## §2 — Seven Tools

All are `core.Tool` values. Sandboxing = closure-internal; core sees only `ToolExecuteFn`.

| # | Tool | Key Schema Fields | Sandbox |
|---|------|------------------|---------|
| 1 | **Read** | `file_path`, `offset`, `limit` (cap: 64KiB) | Path jail: resolve → reject `..` + symlink escapes |
| 2 | **Write** | `file_path`, `content` | Path jail + `os.WriteFile(0644)`, auto-create parents |
| 3 | **Edit** | `file_path`, `old`, `new`, `n` (-1=all) | Path jail + backup to `.tau-backups/` before write |
| 4 | **Bash** | `command`, `work_dir`, `timeout_seconds` (120s) | `exec.CommandContext(ctx, "bash", "-c", cmd)` with env allowlist (`PATH`,`HOME`,`SHELL`), output cap 64KiB |
| 5 | **Glob** | `pattern` (`**/*.go`), `work_dir` | Path jail: walk within workspace root |
| 6 | **Grep** | `pattern` (regexp), `path`, `include` glob, `n` (cap: 100) | Path jail: walk within workspace root |
| 7 | **Task** | `action`, `task_id`, `title` | Session-memory only (no FS), cleared on session end |

**Schema**: codegen via `toolspec/` (same pipeline as `echo.schema.json`). Each tool gets a
`.schema.json` → `go generate` → `ToolSchema` implementing `Marshal()` + `Validate()`.

**Registration**: all 7 at session init, `SetActive` controlled by `--no-tools` flag or dynamic policy.

**Sandbox design**: Bash env-allowlist + workspace-root jail. Container/vm later via `BashOperations`
interface swap (pi pattern). No container needed for v1.

---

## §3 — Skills (G3)

Skills follow pi's `SKILL.md` convention. **No core changes** — `SystemPromptFn` closure is the boundary.

### Format

```markdown
---
name: "rust-refactor"
description: "Rust refactoring: extract function, simplify match, use combinators"
disable-model-invocation: false
---
## Guidelines
- Prefer match over if-let chains...
```

### Loading + Injection

```go
// pkg/coding/skills/loader.go
type Skill struct {
    Name, Description, Content, SourcePath string
    DisableInvocation bool
}
type Loader struct { dirs []string }
func (l *Loader) Load() ([]Skill, error)
```

Closure pattern:

```go
loader := skills.NewLoader(dirs)
systemPrompt := func(sess *core.Session) (string, error) {
    skills, _ := loader.Load()
    return prompts.Build(prompts.Input{
        Skills: skills, WorkspaceRoot: wsRoot,
        Tools: sess.Tools.Active(),
    }), nil
}
```

Skills appear as index in system prompt. LLM invokes skill by name → product layer injects
skill body into next turn's context.

### Discovery Order

1. Built-in: `embed.FS` in `pkg/coding/skills/builtin/`
2. User: `~/.tau/skills/`
3. Project: `<workspace>/.tau/skills/`

Later overrides earlier by `name`.

---

## §4 — System Prompt

### Template Structure (4 blocks + dynamic)

```
[IDENTITY]     — "You are tau, a coding agent. Use tools. Verify before claiming."
[RULES]        — Behavioral rules (verify, don't guess, run tests after changes)
[SKILLS]       — Available skills index (from SkillLoader)
[TOOLS]        — Tool list with schemas (from sess.Tools.Active())
```

### Dynamic Injection (each turn)

```go
sess.Hooks.TransformContext.Add(func(ctx context.Context, msgs []core.Message) ([]core.Message, error) {
    block := formatDynamicBlock(gitStatus(), openFiles())
    return append([]core.Message{{Role: core.RoleSystem, Content: block}}, msgs...), nil
})
```

Dynamic block: git branch + dirty status, open files, recent tool outputs (last 3).

### Prompt Files

`pkg/coding/prompts/` — `system_base.md`, `system_workspace.md`. Embedded via `embed.FS`.
Rendered with `text/template`. Override via `--system-prompt <file>`.

---

## §5 — CLI

### Entry: `cmd/tau/main.go`

```
tau [flags] [prompt...]

--provider <name>       Provider (default: TAU_PROVIDER env → "openai")
--model <model>         Model (default: provider default)
--api-key <key>         API key (env: TAU_API_KEY)
--workspace <dir>       Root (default: cwd)
--system-prompt <file>  Override template
--skills-dir <dir>      Additional skills dir (repeatable)
--no-skills             Disable skills
--no-tools              Disable all tools
--continue, -c          Continue last session
--resume <id>           Resume specific session
--verbose, -v
```

### Flow

1. Parse args → resolve workspace (flag → env → cwd).
2. Load skills (unless `--no-skills`).
3. `sess := core.NewSession(ctx, core.SessionOptions{SystemPrompt: closure, Provider: ..., DefaultModel: ...})`
4. `sess.Providers.RegisterVendor(name, cfg)`
5. Register 7 tools → `sess.Tools`, register hooks (TransformContext, BeforeToolCall for permission gate).
6. `loop := core.NewLoop(); run, _ := loop.Prompt(ctx, sess, input)`
7. Stream `run.Events` → stdout (TUI in Phase 5+). Block on `run.Done()`.

---

## §6 — Core Access Points (11/11 consumed)

| # | AP | Usage |
|---|----|-------|
| 1 | `sess.Tools.Register` | Register 7 tools |
| 2 | `sess.Tools.SetActive` | `--no-tools` flag |
| 3 | `sess.Hooks.BeforeToolCall` | Permission gate (destructive ops) |
| 4 | `sess.Hooks.AfterToolCall` | File-op extraction for compaction |
| 5 | `SystemPromptFn` | Skills + prompt injection |
| 6 | `run.Events` | Stream to stdout / TUI |
| 7 | `Transcript.Subscribe` | Multi-client webui (Phase 5+) |
| 8 | `run.Done()` | Batch mode completion |
| 9 | `core.NewSession` | Per-invocation session |
| 10 | `sess.Providers.RegisterVendor` | Provider registration |
| 11 | `Provider.Complete` | Compaction summarization |

Zero new core access points required.

---

## §7 — Package Layout

```
cmd/tau/main.go                     CLI entry
pkg/coding/
  tools/{read,write,edit,bash,glob,grep,task,sandbox}.go
  skills/{loader,format}.go + builtin/*.md
  prompts/{base,workspace}.md + prompts.go
  session.go                        NewCodingSession(opts)
```

---

## §8 — Out of Scope (v1)

| Item | Status | Rationale |
|------|--------|-----------|
| TUI | Deferred | Batch mode first; `run.Events` already supports streaming |
| WebUI | Deferred | After TUI |
| Branching | Product-layer tree on flat Transcript | Design note in `01-core-design.md` §3 |
| Container sandbox | Deferred | v1 = process jail; `BashOperations` interface planned for swap |
| Extension system | Deferred | Wait for real-world usage data |

---

## §9 — Core Gaps

**None.** All capabilities map to existing abstractions:
- Tools → `core.Tool` + `ToolRegistry`
- Skills → `SystemPromptFn` closure (G3-designed for this)
- Dynamic context → `HookSet.TransformContext`
- CLI session → `core.NewSession` + `Loop.Prompt` (demo-verified)
- Sandbox → `ToolExecuteFn` closure (ctx carries cancellation; closure captures workspace root)

**Zero core source file modifications required.**

---

*09-coding-agent.md — 7 tools, skills via closure, prompt template + TransformContext, CLI flags, zero core changes.*

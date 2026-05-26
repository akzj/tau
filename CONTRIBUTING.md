# Contributing to tau

## How to Contribute

1. **Fork** the repository
2. **Create a branch**: `feature/your-feature` or `fix/your-fix`
3. **Make changes**: write code, add tests, update docs
4. **Run tests**: `go test -count=1 ./...`
5. **Submit a PR**: describe what changed, why, and how to test

## Development Setup

```bash
# Requirements: Go 1.23+
go version

# Clone
git clone https://github.com/akzj/tau
cd tau

# Build
go build -o tau ./cmd/tau/

# Run all tests (zero API keys needed — uses faux provider)
go test -count=1 ./core/ ./pkg/testing/faux/ ./pkg/coding/tools/ ./pkg/persist/ ./pkg/extensions/ ./providers/...

# Run coding agent (needs ANTHROPIC_AUTH_TOKEN for LLM access)
echo "say hello" | ./tau --max-turns 1
```

## Architecture

See [AGENTS.md](./AGENTS.md) for the full architecture tree and capability matrix.

```
core/          Agent runtime kernel (Loop, Session, Transcript, Tool, Hook, Provider)
providers/     9 wire protocols (OpenAI, Anthropic, Google, Azure, Mistral, Bedrock, Vertex, Responses, Codex)
pkg/coding/    Product layer: 10 tools + 8 skills + system prompts + session management
pkg/tui/       Terminal UI (Bubble Tea)
pkg/webui/     Web UI (HTTP + WebSocket)
pkg/extensions/ goja JS VM runtime
pkg/persist/   Session persistence (JSONL)
pkg/sandbox/   Docker/Podman container sandbox
pkg/testing/   Faux provider (zero API keys for testing)
```

## Adding a New Tool

1. Create `pkg/coding/tools/your_tool.go` — return a `core.Tool{Name, Description, Schema, Execute}`.
2. Use `ResolvePath(args.FilePath)` for path-jail sandboxing.
3. Register in `pkg/coding/session.go` → `NewCodingSession()` + `SetActive` list.
4. Add E2E test in `pkg/coding/tools/tools_e2e_test.go` using faux provider.
5. Run `go test ./pkg/coding/tools/ -v -count=1`.

## Adding a New Provider

1. Create `providers/your-provider/provider.go` — implement `core.Provider` (Stream + Complete).
2. Add models to `pkg/provider/models.json`.
3. Register factory in `pkg/provider/lazy.go` → `NewProviderLoader()`.
4. Add `//go:build !no_yourprovider` constraint for opt-out.
5. Wire `--model` auto-routing in `cmd/tau/main.go`.

## Adding a New Skill

1. Create `pkg/coding/skills/builtin/your-skill.md` with frontmatter:
   ```markdown
   ---
   name: "your-skill"
   description: "What it does"
   ---
   ## Guidelines
   ...
   ```
2. Skills are auto-discovered by the `SystemPromptFn` closure — no code registration needed.

## Code Style

- **gofmt**: All code must be `gofmt`-formatted. CI enforces this.
- **go vet**: Zero warnings. Run `go vet ./...` before committing.
- **Godoc**: Every exported symbol must have a one-line godoc comment: `// Name does X.`
- **Imports**: Standard library first, then third-party, then internal packages.
- **Errors**: Use `fmt.Errorf("context: %w", err)` for wrapping. Never discard errors silently.
- **No `any` in public API**: Core interfaces are sealed — use typed interfaces, not `interface{}`.

## Testing

- **Unit tests**: Standard `go test`. Run per package.
- **E2E tests**: Use `faux` provider (zero API keys). See `pkg/coding/tools/tools_e2e_test.go` for examples.
- **Coverage**: Aim for >80% on new code. `go test -coverprofile=/tmp/cover.out ./pkg/...`
- **Integration**: Mark with `//go:build integration`. Run manually or nightly.

## Design Principles

tau follows 6 iron laws (R1–R6) distilled from pi:
- **R1**: Types = runtime truth. Zero `any` in public API.
- **R2**: Single canonical abstraction per concept.
- **R3**: Every rule must be lint-enforceable.
- **R4**: Registry must be scoped (no global mutable state).
- **R5**: Core never knows product details (tool names, provider names).
- **R6**: Design philosophy must be first-class documentation.

See `docs/v2/04-design-principles.md` for the full text.

## Issue Guidelines

- **Bug reports**: Include reproduction steps, expected vs actual behavior, `go version`, tau commit hash.
- **Feature requests**: Describe the use case and how it fits tau's scope (agent runtime, not full product).
- **PRs**: Reference related issues. Keep changes focused — one concern per PR.

## License

MIT — see [LICENSE](./LICENSE).

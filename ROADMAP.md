# tau Roadmap

## v0.1.0 (Current)

tau is a production-ready agent runtime with full infrastructure.

- **9 wire protocols**: OpenAI Completions, Anthropic Messages, Google GenAI, Azure OpenAI, Mistral, Bedrock (AWS), Vertex AI, OpenAI Responses, Codex Responses
- **16 tools**: read, write, edit, bash, glob, grep, task, task_tracker, web_search, web_fetch, workspace_diag, list_files, search_code, run_tests, git_diff, ask_user
- **20 skills**: 8 general + 4 Go + 4 Python + 4 JS/TS — 3-layer discovery (builtin → user → project)
- **3 UIs**: batch CLI, Bubble Tea TUI (syntax highlighting, file tree, keyboard shortcuts), WebUI (dark theme, responsive, session management)
- **Production infrastructure**: graceful shutdown, health checks, Prometheus metrics, structured logging, config file (tau.yaml), session persistence (JSONL), container sandbox (Docker/Podman), extension system (goja JS VM), CI/CD pipeline, Docker deployment, version embedding

See [CHANGELOG.md](./CHANGELOG.md) for the full feature list.

## v0.2.0 (Planned)

- **Plugin system**: external tool packages, dynamic loading, plugin marketplace
- **20+ tools**: image generation, audio transcription, database query, Jupyter notebook
- **Streaming optimization**: token-level backpressure, adaptive chunking, reduced allocs
- **2× test coverage**: provider integration tests, TUI screenshot tests, load testing
- **Cross-platform TUI**: Windows terminal support, mouse mode on all platforms

## v0.3.0 (Planned)

- **MCP (Model Context Protocol)**: server/client support, resource discovery, tool proxy
- **Multi-agent**: agent-to-agent communication, sub-agent delegation, agent teams
- **gRPC API**: streaming agent service, session management, health + metrics endpoints
- **Auth & RBAC**: API key management, per-user sessions, tool permission policies
- **Observability**: OpenTelemetry tracing, structured log aggregation, Grafana dashboards

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for how to add new tools, providers, and skills.

## Versioning

tau follows [Semantic Versioning](https://semver.org/). The public API surface is:
- CLI flags and commands
- Extension API (goja `tau.*` host object)
- Config file format (tau.yaml)
- Session file format (JSONL)
- Provider endpoint contracts

# tau Usage Guide

## Quick Start

```bash
# Build
go build -o tau ./cmd/tau/

# Set API token (required for LLM access)
export ANTHROPIC_AUTH_TOKEN=your_token

# Single turn
echo "What is Go?" | ./tau --max-turns 1

# Multi-turn (session persists automatically)
echo "My name is Alice" | ./tau --max-turns 2
echo "What's my name?" | ./tau --resume <session-id> --max-turns 1
```

## Provider Configuration

```bash
# List available models
./tau --list-models
# MODEL                  PROVIDER     CTX WIN  COST IN      COST OUT
# gemini-2.5-flash       google       1000000  $0.15        $0.60
# gpt-4o                 openai       128000   $2.50        $10.00
# claude-sonnet-4-6      anthropic    200000   $3.00        $15.00
# ...

# Use specific model/provider
echo "Hello" | ./tau --model claude-sonnet-4-6 --max-turns 1
echo "Hello" | ./tau --model gemini-2.5-flash --max-turns 1

# Config file (~/.tau/tau.yaml)
cat > ~/.tau/tau.yaml << 'EOF'
provider: anthropic
model: claude-sonnet-4-6
log_level: debug
EOF
```

## Tools

```bash
# List all tools
./tau --list-tools
# Available tools:
#   read  write  edit  bash  glob  grep
#   task  task_tracker  web_search  web_fetch
#   workspace_diag  list_files  search_code
#   run_tests  git_diff  ask_user
#   lint  format  deps  coverage

# Tool use in practice
echo "Read core/loop.go, find all TODOs, and list them" | ./tau --max-turns 4

# Restrict tools
./tau --no-tools "What is Go?"  # LLM-only, no tools

# Enable specific tools via code
sess.SetTools([]string{"read", "grep", "write"})
```

## Skills

```bash
# List available skills
./tau --list-skills
# Available skills (built-in):
#   code-review  debugger  test-writer  refactor  architect
#   go-refactor  shell-scripting  git-workflow
#   go-code-review  go-debugging  go-test-writing  go-refactoring
#   python-code-review  python-debugging  python-test-writing  python-refactoring
#   js-code-review  js-debugging  js-test-writing  js-refactoring
#   refactoring-patterns  debugging-strategies  api-design
#   database-patterns  cicd-patterns  testing-strategy
#   security-review  code-review-intensive
#   performance-optimization  documentation-generation

# Skills auto-inject — no flag needed
echo "Review this code for security issues" | ./tau --max-turns 5

# Custom skills directory
./tau --skills-dir ~/my-skills "Use my custom skill"
```

## Session Management

```bash
# Sessions save automatically
echo "Hello" | ./tau --max-turns 1
# → [saved 20250601-120000] (2 messages)

# List sessions
./tau --list-sessions

# Resume a session
echo "Continue where we left off" | ./tau --resume 20250601-120000 --max-turns 2

# Steer (mid-turn direction)
./tau --resume 20250601-120000 --steer "switch to Python" --max-turns 1
```

## Plugins

```bash
# Build and install a plugin
go build -o /tmp/plugin-workspace-diag ./cmd/plugin-workspace-diag/
./tau plugin install /tmp/plugin-workspace-diag

# Manage plugins
./tau plugin list
./tau plugin info plugin-workspace-diag
./tau plugin remove plugin-workspace-diag
```

## Multi-Agent

```go
// Go code — spawn 3 sub-agents in parallel
pool := core.NewSubAgentPool()
pool.Spawn(core.SubAgentSpec{ID: "worker-1", Prompt: "...", Budget: 3})
pool.Spawn(core.SubAgentSpec{ID: "worker-2", Prompt: "...", Budget: 3})
results, _ := pool.Collect(ctx)
```

## Docker

```bash
# Build and run with Docker
docker build -t tau .
echo "Hello from Docker" | docker run -i -e ANTHROPIC_AUTH_TOKEN tau --max-turns 1

# Docker Compose
docker-compose up
curl http://localhost:8080/health
```

## Production

```bash
# Health check
./tau --health-addr :8080 &
curl http://localhost:8080/health
# → {"status":"ok","version":"dev","uptime":"5s","providers":7,"tools":20}

# Prometheus metrics
./tau --metrics-addr :9090 &
curl http://localhost:9090/metrics

# Graceful shutdown
kill -TERM <pid>
# → session saved, 10s deadline, second signal force-exit

# Logging
./tau --log-level debug --log-format json "Debug mode"

# Docker deployment
docker run -d --name tau \
  -e ANTHROPIC_AUTH_TOKEN=$ANTHROPIC_AUTH_TOKEN \
  -p 8080:8080 -p 9090:9090 \
  tau --webui --addr :8080 --health-addr :8081 --metrics-addr :9090
```

## TUI

```bash
# Launch terminal UI
./tau --tui

# Keyboard shortcuts:
# Ctrl+C quit | Ctrl+L clear | Ctrl+S save | Ctrl+R resume
# Ctrl+N new session | Ctrl+P cycle provider | Ctrl+M cycle model
# Ctrl+T toggle file tree | Mouse wheel scroll
```

## WebUI

```bash
# Launch web UI
./tau --webui --addr :8080
# Open http://localhost:8080

# File tree sidebar: toggle with 📁 Files
# Session dropdown: select/resume saved sessions
# Responsive: works on mobile (≤480px)
```

## Development

```bash
# Run all tests (zero API keys — faux provider)
make test

# Run benchmarks
make bench

# Run with race detector
go test -race -count=1 ./...

# Build release binary
make build VERSION=v1.0.0
./tau --version
# → tau v1.0.0 (built 2025-07-01_12:00:00, commit abc1234)
```

## Testing (Zero API Keys)

```bash
# All tests use faux provider — no API keys required
go test -count=1 ./...

# Coverage
go test -cover ./...

# E2E tests (requires tau binary)
TAU_BIN=./tau make test-e2e

# Integration tests (requires API keys)
make test-integration
```
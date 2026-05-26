---
name: "go-debugging"
description: "Go debugging: pprof, delve, race detector, memory profiles"
trigger: "debug|profile|race|leak go|golang"
language: go
---

## Go Debugging Tools
- **Race detector**: `go test -race ./...` or `go run -race ./cmd/tau/`. Finds all data races. Mandatory before any concurrent code PR.
- **pprof CPU**: `import _ "net/http/pprof"` + `go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30`
- **pprof memory**: `go tool pprof http://localhost:6060/debug/pprof/heap` — find memory leaks.
- **Delve (delve)**: `dlv debug ./cmd/tau/` → breakpoints, watch variables, step through goroutines.
- **Goroutine dump**: `kill -SIGQUIT <pid>` or `http://localhost:6060/debug/pprof/goroutine?debug=2`
- **Stack traces**: `runtime/debug.Stack()` for in-code stack dumps.

## Process
1. Add `-race` flag to tests: `go test -race -count=1 ./pkg/...`
2. If suspicious, add pprof endpoint temporarily, reproduce the issue.
3. Use `go tool pprof` to identify hot paths or leaks.
4. Fix the root cause, not the symptom.
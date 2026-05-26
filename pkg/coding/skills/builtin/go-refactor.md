---
name: "go-refactor"
description: "Go refactoring: extract functions, simplify error handling, use idiomatic patterns"
disable-model-invocation: false
---

## Guidelines
- **Error handling**: Always wrap errors with context using `fmt.Errorf("context: %w", err)`. Never silently discard errors.
- **Early returns**: Prefer early returns over deep nesting. If err != nil, return immediately.
- **Named returns**: Avoid named return values unless required for deferred error handling.
- **Interfaces**: Define interfaces where they're consumed, not where they're implemented. Keep interfaces small (1-3 methods).
- **Context**: Always pass `context.Context` as the first parameter to functions that do I/O.
- **Testing**: Use table-driven tests. Test edge cases: empty input, nil, timeout.
- **Concurrency**: Use `sync.WaitGroup` for goroutine coordination. Protect shared state with `sync.Mutex`. Prefer channels for communication.
- **Dependencies**: No unnecessary imports. Remove unused imports after refactoring.

---
name: "go-code-review"
description: "Go code review: goroutines, channels, contexts, error handling, interfaces"
trigger: "review|check|audit go|golang code"
language: go
---

## Go-Specific Checks
- **Goroutine leaks**: Every goroutine must have a defined exit path. Use `context.Context` for cancellation.
- **Channel discipline**: Close only from sender. Check for unbuffered channel deadlocks. Prefer `select` with `default` or timeout.
- **Context propagation**: All I/O functions must accept `context.Context` as first parameter. Never use `context.Background()` in library code.
- **Error wrapping**: Use `fmt.Errorf("context: %w", err)`. Never use `%v` for errors that will be inspected with `errors.Is`/`errors.As`.
- **Interface design**: Define interfaces where consumed, not implemented. Keep interfaces small (1–3 methods). Accept interfaces, return structs.
- **Mutex hygiene**: `defer mu.Unlock()` immediately after `mu.Lock()`. Never copy a `sync.Mutex`.
- **nil safety**: Check `nil` before type assertion. `nil` interface != `nil` concrete type.
- **defer order**: Deferred calls run LIFO. Close resources in reverse order of acquisition.
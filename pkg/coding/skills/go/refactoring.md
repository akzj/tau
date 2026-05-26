---
name: "go-refactoring"
description: "Go refactoring: interface extraction, named returns removal, error handling simplification"
trigger: "refactor|clean|simplify|extract go|golang"
language: go
---

## Go Refactoring Patterns
- **Extract interface**: If a function takes a concrete type, extract an interface with only the methods it uses. Place the interface at the call site.
- **Remove named returns**: Named returns (`func f() (result error)`) obscure control flow. Replace with explicit returns unless required for `defer` error modification.
- **Simplify error handling**: Chain `if err != nil { return ... }` early. Avoid nested error checks.
- **Use `errors.Is`/`errors.As`**: Replace `err == SomeErr` with `errors.Is(err, SomeErr)`.
- **Struct embedding**: Use embedding to compose behavior, not for inheritance. Prefer explicit fields when in doubt.
- **Functional options**: Replace constructor parameter explosion with `type Option func(*Config)` pattern.
- **Context threading**: Ensure every I/O call has ctx parameter. Don't store context in structs.

## Process
1. Run tests: `go test -race ./pkg/...` — establish baseline.
2. Make one refactoring change at a time.
3. Run tests after each change.
4. If tests fail, revert and try a smaller step.
---
name: "test-writer"
description: "Write comprehensive tests: table-driven, edge cases, parallel, mocks"
disable-model-invocation: false
trigger: "test|testing|coverage|mock|stub"
tools: [read, write, edit, bash, grep, glob]
---

## Patterns
- **Table-driven**: `[]struct{name string, input X, want Y}` — one test function, many cases.
- **Edge cases**: Empty input, nil, zero value, max value, timeout, cancelled context, concurrent access.
- **Error paths**: Test what happens when a file doesn't exist, when a command fails, when the network is down.
- **Parallel**: Use `t.Parallel()` for independent tests. Speeds up test suite significantly.
- **Cleanup**: Use `t.Cleanup()` and `defer` for temp files and resources.

## Mocks
- Mock at package boundaries via interfaces. Don't mock internal functions.
- Use `faux` provider for LLM tests (zero API keys).
- For HTTP: use `httptest.NewServer`.

## Process
1. Read the function you're testing — understand all code paths.
2. Write a table-driven test covering: happy path, each error path, edge cases.
3. Run `go test -v -count=1 ./pkg/...` to verify.
4. Check coverage: `go test -coverprofile=/tmp/cover.out ./pkg/...`
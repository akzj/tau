---
name: "go-test-writing"
description: "Go test writing: table-driven tests, subtests, mocking, coverage"
trigger: "test|cover|mock go|golang"
language: go
---

## Go Test Patterns
- **Table-driven**: `tests := []struct{name string; input X; want Y}{...}` + `for _, tt := range tests { t.Run(tt.name, func(t *testing.T) {...}) }`
- **Subtests**: `t.Run("subtest name", func(t *testing.T) { ... })` — isolates setup/teardown per case.
- **Parallel**: `t.Parallel()` for independent tests. Greatly speeds up test suite.
- **Golden files**: Use `testdata/` directory + `os.ReadFile` / `os.WriteFile` for expected output snapshots.
- **Mocking**: Define interfaces at package boundaries. Use `faux` provider for LLM-dependent tests.
- **Temp files**: `t.TempDir()` — auto-cleaned after test.
- **Helper functions**: Mark with `t.Helper()` so failures point to caller, not helper.

## Process
1. Read the function signature and all code paths.
2. Write table covering: happy path, each error path, edge cases (nil, empty, max, timeout).
3. Run `go test -race -coverprofile=/tmp/cover.out ./pkg/...`
4. Use `go tool cover -html=/tmp/cover.out` to find uncovered lines.
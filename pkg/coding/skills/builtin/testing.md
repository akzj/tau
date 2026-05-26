---
name: "testing"
description: "Test writing: table-driven tests, edge cases, mocks, coverage"
disable-model-invocation: false
---

## Guidelines
- **Table-driven tests**: Use `[]struct{name, input, want}` pattern. One test function, many cases.
- **Edge cases**: Empty input, nil, zero values, max values, timeout, cancelled context.
- **Test error paths**: Not just happy path. What happens when the file doesn't exist? When the command fails?
- **Clean up**: Use `t.Cleanup()` for temp files, `defer` for resources. Don't leave state behind.
- **Parallel tests**: Use `t.Parallel()` for independent tests. Speeds up test suite.
- **Coverage**: Aim for >80% on new code. Use `go test -coverprofile` to check.
- **Integration tests**: Separate from unit tests with build tags (`//go:build integration`). Run unit tests in CI, integration tests manually or nightly.
- **Mock at boundaries**: Mock external services (APIs, databases), not internal functions. Use interfaces for testability.
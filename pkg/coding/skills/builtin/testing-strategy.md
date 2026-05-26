---
name: "testing-strategy"
description: "Testing strategy: test pyramid, mock vs fake, table-driven, snapshot, E2E"
disable-model-invocation: false
trigger: "test strategy|test plan|coverage|test pyramid|testing approach"
tools: [read, write, edit, bash, grep, glob]
---

## Strategy
- **Test Pyramid**: Many unit tests (fast) → fewer integration tests → few E2E tests (slow). Invest at the bottom.
- **Mock vs Fake**: Mock = verify interaction (`mock.AssertCalled`). Fake = real implementation with simplified behavior (faux provider). Prefer fakes.
- **Table-driven**: One test function, many test cases. `[]struct{name, input, want}`. Go standard pattern.
- **Snapshot**: Record expected output to file. Compare on next run. Great for API responses and rendered output.
- **E2E**: Test the full system with faux dependencies. Slower but catches integration bugs.

## Steps
1. Write unit tests for new logic. Cover happy path + each error path + edge cases.
2. Write integration tests for database/service boundaries. Use test containers or in-memory substitutes.
3. Write one E2E test for the critical user journey.
4. Run coverage: `go test -cover ./...` Aim for >80% on new code.
5. Run race detector: `go test -race ./...`

---
name: "code-review-intensive"
description: "In-depth code review: checklist approach, nit vs blocker, incremental review, author guide"
disable-model-invocation: false
trigger: "thorough review|deep review|intensive review|PR review|merge request"
tools: [read, grep, glob, bash]
---

## Review Dimensions
- **Correctness**: Does the code do what it claims? Are edge cases handled (nil, empty, timeout, concurrent)?
- **Security**: Refer to `security-review` skill. Check auth, input validation, secrets, SQL injection.
- **Performance**: N+1 queries? Unnecessary allocations? Blocking in hot paths? Memory leaks?
- **Maintainability**: Is the code readable? Are names descriptive? Is complexity manageable?
- **Testing**: Are there tests for the new code? Do tests cover error paths and edge cases?

## Nit vs Blocker
- **Blocker**: Security vulnerability, data loss risk, incorrect behavior, missing error handling, race condition. Must fix before merge.
- **Nit**: Naming preference, style choice, minor refactor. Suggest but don't block.

## Steps
1. Read the PR description. Understand what the change is supposed to do.
2. Read the diff from top to bottom. Note any surprises.
3. Check out the branch and run tests locally: `go test -race ./...`
4. Leave comments: blocker (must fix), suggestion (consider), question (clarify).
5. Approve only when all blockers are resolved.

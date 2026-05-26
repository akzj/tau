---
name: "code-review"
description: "Review code for bugs, logic errors, security issues, performance, and style"
disable-model-invocation: false
trigger: "review|check|audit|inspect code"
tools: [read, grep, glob]
---

## Guidelines
- **Logic**: Check for off-by-one, nil pointer, race conditions, deadlocks.
- **Bugs**: Unhandled errors, missing nil checks, incorrect error wrapping, context cancellation not honored.
- **Security**: SQL injection, XSS, hardcoded secrets, missing auth, path traversal.
- **Performance**: N+1 queries, unnecessary allocations, blocking in hot paths, missing indexes.
- **Style**: Consistent naming, idiomatic patterns, proper error messages, DRY violations.
- **Process**: Read the changed files first → grep for related callers → report findings with file:line references.
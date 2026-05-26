---
name: "refactoring-patterns"
description: "Extract, inline, rename, and move code patterns for systematic refactoring"
disable-model-invocation: false
trigger: "refactor|restructure|clean up|extract|inline code"
tools: [read, write, edit, bash, grep, glob]
---

## Patterns
- **Extract Function**: Move a code block into a named function. Look for: repeated logic, long functions (>30 lines), nested conditionals.
- **Inline Function**: Replace a function call with its body. When: the function body is clearer than the name.
- **Rename**: Change variable/function/type names to reveal intent. Names should describe WHAT, not HOW.
- **Move**: Relocate a function/field to where it's used. Dependencies should flow inward.
- **Replace Conditional with Polymorphism**: Switch/if chains on type codes → interface implementations.

## Steps
1. Run existing tests to establish baseline: `go test ./pkg/...`
2. Make one pattern change at a time.
3. Run tests after each change. If tests fail, revert and try a smaller step.
4. Commit when all tests pass.

## Example
```
Before: 50-line function doing 3 things
Step 1: Extract helper for thing 1 (10 lines) → test
Step 2: Extract helper for thing 2 (15 lines) → test
After: 15-line orchestrator + 3 focused helpers
```

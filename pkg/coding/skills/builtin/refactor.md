---
name: "refactor"
description: "Refactor code: extract functions, simplify logic, rename, restructure without changing behavior"
disable-model-invocation: false
trigger: "refactor|clean|simplify|extract|rename|restructure"
tools: [read, write, edit, grep, glob, bash]
---

## Principles
- **No behavior change**: The refactored code must produce identical outputs for identical inputs.
- **Small steps**: Make one refactoring change at a time. Test after each change.
- **Extract, don't expand**: Break large functions into smaller, named functions. Keep each function focused.
- **Simplify conditions**: Replace nested if-else with early returns. Use switch over long if chains.
- **Rename for clarity**: Names should describe WHAT, not HOW. `fetchUserByID` not `getData`.

## Process
1. Read the target code carefully. Understand every code path.
2. Run existing tests to establish baseline: `go test ./pkg/...`
3. Make one refactoring change (extract, rename, simplify).
4. Run tests again. If they fail, revert and try a smaller step.
5. Repeat until the code is clean and tests still pass.
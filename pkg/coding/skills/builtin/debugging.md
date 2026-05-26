---
name: "debugging"
description: "Systematic debugging approach: isolate, reproduce, fix, verify"
disable-model-invocation: false
---

## Guidelines
- **Read the error first**: Don't guess. The error message tells you what happened and where.
- **Isolate**: Find the minimum reproduction. Remove variables one at a time.
- **Add logging before fixing**: `fmt.Printf` or `log.Printf` at the failure point to see actual values.
- **Binary search**: If you don't know where the bug is, bisect: comment out half the code, test. Repeat.
- **Check assumptions**: Is the file where you think it is? Is the variable what you think it is? Verify with `read` or `bash`.
- **One fix at a time**: Make one change, test, then move on. Multiple changes hide which one worked.
- **Write a test for the bug**: After fixing, add a test that reproduces the original failure. Prevents regression.
- **Check git history**: `git log --oneline -20` and `git diff` to see what changed recently.
---
name: "debugger"
description: "Systematic debugging: isolate, reproduce, diagnose, fix, verify"
disable-model-invocation: false
trigger: "debug|bug|error|crash|fail|broke"
tools: [read, grep, glob, bash, edit, write]
---

## Process
1. **Isolate**: Find the minimum reproduction. Where exactly does it fail? What's the error message?
2. **Reproduce**: Add logging/prints at the failure point. Run the failing command. Confirm you can reproduce it.
3. **Diagnose**: Read the code at the failure point. Trace the call chain backwards. Check assumptions: is the file there? Is the variable what you think?
4. **Fix**: Make the smallest possible change. One fix at a time.
5. **Verify**: Run `go build ./...` and any relevant tests. Confirm the error is gone.

## Tips
- Use `grep` to find where the error is defined or thrown
- Use `glob` to find related files by pattern
- Use `bash` to run the failing command and see the exact error output
- Binary search: comment out half the code, test, repeat until you find the line
- Check git log for recent changes: `git log --oneline -20`
# Code Review — Grep + Read + Analyze

Review code in the workspace.

## Command
```bash
echo "Find all functions that don't check errors in Go files. List the file and line numbers." | ./tau --max-turns 4
```

## Expected Flow
```
Turn 1: [tool: glob] → *.go files found: 5 files
Turn 2: [tool: grep] → matches for pattern "err != nil"
Turn 3: [tool: read] → context around each match
Turn 4: Analysis: found 3 functions without error checks in main.go:42, handler.go:156...
```

## What's happening
1. tau finds Go files with `glob`
2. Searches error handling patterns with `grep`
3. Reads context around matches
4. Reports findings

## Explore More
```bash
# Review with the code-review skill
echo "Review this project for security issues" | ./tau --max-turns 5
```

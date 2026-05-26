# Parallel Tools — Read Multiple Files

tau executes independent tool calls in parallel.

## Command
```bash
echo "Read core/event.go, core/tool.go, and core/loop.go and tell me how they relate" | ./tau --max-turns 3
```

## Expected Flow
```
Turn 1: [tool: read] core/event.go → [tool: read] core/tool.go → [tool: read] core/loop.go
Turn 2: Analysis: event.go defines the AgentEvent sealed interface...
```

## What's happening
1. tau reads all three files in parallel (WaitGroup)
2. Results are emitted in deterministic order (by call ID)
3. tau analyzes the relationships between files

## Performance Note
Parallel tool execution means reading 3 files takes ~the same time as reading 1 file. Sequential tools (like bash) are executed after parallel tools complete.

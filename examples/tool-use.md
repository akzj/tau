# Tool Use — Read, Write, Bash

tau's coding tools in action.

## Command
```bash
echo "Create a file called hello.go with a main function that prints 'Hello, tau!'. Then run it." | ./tau --max-turns 5
```

## Expected Flow
```
Turn 1: [tool: write] → Wrote N bytes to hello.go
Turn 2: [tool: bash] → go run hello.go
Turn 3: Hello, tau!
```

## What's happening
1. tau uses the `write` tool to create `hello.go`
2. tau uses the `bash` tool to run `go run hello.go`
3. tau reports the output

## Explore More
```bash
# List all available tools
./tau --list-tools

# Run with specific tools only
echo "read the file hello.go" | ./tau --max-turns 2
```

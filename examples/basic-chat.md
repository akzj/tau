# Basic Chat — Single Turn

Ask tau a question and get a response.

## Command
```bash
echo "What is the difference between goroutines and threads?" | ./tau --max-turns 1
```

## Expected Output
```
[turn 1]
[turn start]
Goroutines are lightweight user-space threads managed by the Go runtime...
[turn end] complete
```

## What's happening
1. tau creates a session with default tools
2. Sends your prompt to the LLM (default: gpt-5.4)
3. Streams the response back
4. Saves the session automatically

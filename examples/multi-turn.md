# Multi-Turn — Session Resume

Keep context across multiple conversations.

## Step 1: First conversation
```bash
echo "My name is Alice and I write Go code" | ./tau --max-turns 2
```
Note the session ID: `[saved 20250601-120000]`

## Step 2: List sessions
```bash
./tau --list-sessions
```

## Step 3: Resume with context
```bash
echo "What is my name and what language do I use?" | ./tau --resume 20250601-120000 --max-turns 2
```

## Expected Output (Step 3)
```
Your name is Alice and you write Go code.
```

## What's happening
tau saves each session as JSONL. `--resume` loads the transcript, so the LLM remembers previous context.

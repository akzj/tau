# tau Examples

Practical examples for getting started with tau.

## Prerequisites
```bash
go build -o tau ./cmd/tau/
export ANTHROPIC_AUTH_TOKEN=your_token  # or MISTRAL_API_KEY, etc.
```

## Examples

| # | Example | What you'll learn |
|---|---------|-------------------|
| 1 | [basic-chat](./basic-chat.md) | Single-turn conversation |
| 2 | [multi-turn](./multi-turn.md) | Session persistence with `--resume` |
| 3 | [tool-use](./tool-use.md) | Read, write, and bash tools |
| 4 | [web-search](./web-search.md) | Web search + summarization |
| 5 | [code-review](./code-review.md) | grep + read + code analysis |
| 6 | [parallel-tools](./parallel-tools.md) | Parallel file reads |

## Quick Test
```bash
# Verify tau works
echo "say hello" | ./tau --max-turns 1

# List available models
./tau --list-models

# Run with different model
echo "explain Go interfaces" | ./tau --model claude-sonnet-4-6 --max-turns 2
```

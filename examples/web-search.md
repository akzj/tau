# Web Search — DuckDuckGo + Summarize

Search the web and summarize results.

## Command
```bash
echo "Search the web for 'Go 1.23 iter package' and summarize what you find" | ./tau --max-turns 3
```

## Expected Flow
```
Turn 1: [tool: web_search] → DuckDuckGo results for "Go 1.23 iter package"
Turn 2: Summary of the iter package: ranges, Seq, Seq2...
[turn end] complete
```

## What's happening
1. tau calls the `web_search` tool (DuckDuckGo Instant Answer API, zero API keys)
2. Reads the abstract and related topics
3. Summarizes the findings

## Tip
Web search is free — no API key needed for DuckDuckGo.

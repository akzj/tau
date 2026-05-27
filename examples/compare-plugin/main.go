package main

import (
	"fmt"
	"os"
)

func main() {
	// Read prompt from args or use default
	prompt := "Say hello in one sentence"
	if len(os.Args) > 1 {
		prompt = os.Args[1]
	}

	cfg := CompareConfig{
		Providers: []string{"openai", "anthropic", "google"},
		Prompt:    prompt,
		Timeout:   0, // use default 30s
	}

	result := RunCompare(cfg)
	fmt.Print(FormatTable(result))
}

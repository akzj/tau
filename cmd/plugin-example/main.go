package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		var resp map[string]any

		switch req.Method {
		case "tools":
			resp = map[string]any{
				"tools": []map[string]any{{
					"name":        "plugin-echo",
					"description": "Echo tool from external plugin",
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"text": map[string]any{"type": "string"},
						},
					},
				}},
				"name":    "plugin-example",
				"version": "0.1.0",
			}
		case "execute":
			toolName, _ := req.Params["tool"].(string)
			if toolName == "plugin-echo" {
				args, _ := req.Params["args"].(map[string]any)
				text, _ := args["text"].(string)
				resp = map[string]any{
					"result": "echo: " + text,
				}
			} else {
				resp = map[string]any{"error": "unknown tool: " + toolName}
			}
		default:
			resp = map[string]any{"error": "unknown method: " + req.Method}
		}

		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}

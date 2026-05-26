// tau plugin template — JSON-RPC stdin/stdout server
// Copy and modify to create your own plugin.

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
			// Return your tool definitions here
			resp = map[string]any{
				"name":    "hello-plugin",
				"version": "0.1.0",
				"tools": []map[string]any{
					{
						"name":        "hello",
						"description": "Say hello to someone",
						"schema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name": map[string]any{
									"type":        "string",
									"description": "Name to greet",
								},
							},
						},
					},
				},
			}

		case "execute":
			// Handle tool execution
			toolName, _ := req.Params["tool"].(string)
			args, _ := req.Params["args"].(map[string]any)

			switch toolName {
			case "hello":
				name, _ := args["name"].(string)
				if name == "" {
					name = "world"
				}
				resp = map[string]any{"result": "Hello, " + name + "!"}
			default:
				resp = map[string]any{"error": "unknown tool: " + toolName}
			}

		default:
			resp = map[string]any{"error": "unknown method: " + req.Method}
		}

		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}

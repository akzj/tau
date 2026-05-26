# tau Plugin SDK

## Overview

tau plugins are standalone binaries that communicate via JSON-RPC over stdin/stdout.
Each plugin implements a `tools` handshake and `execute` method.
Plugins run as subprocesses — any language works.

## Quick Start

```bash
# 1. Create a plugin directory
mkdir my-plugin && cd my-plugin
cp -r $TAU_HOME/examples/plugin-template/* .

# 2. Implement your tool logic in main.go

# 3. Build
make build

# 4. Install
tau plugin install ./my-plugin

# 5. Verify
tau plugin list
tau plugin info my-plugin
```

## Protocol Reference

### Transport
- **stdin**: JSON-RPC requests (one per line, newline-delimited)
- **stdout**: JSON-RPC responses (one per line, newline-delimited)
- **stderr**: Free-form logging (displayed to user)

### Methods

#### `tools` — Handshake

Request:
```json
{"method":"tools","params":null}
```

Response:
```json
{
  "name": "my-plugin",
  "version": "0.1.0",
  "tools": [
    {
      "name": "my-tool",
      "description": "What this tool does",
      "schema": {
        "type": "object",
        "properties": {
          "input": {"type": "string", "description": "Input text"}
        }
      }
    }
  ]
}
```

#### `execute` — Run a Tool

Request:
```json
{"method":"execute","params":{"tool":"my-tool","args":{"input":"hello"}}}
```

Response:
```json
{"result":"Tool output: hello"}
```

Error:
```json
{"error":"something went wrong"}
```

### Lifecycle

1. tau discovers the binary in `$TAU_PLUGIN_DIR` (or `~/.tau/plugins/`)
2. tau spawns the binary as a subprocess
3. tau sends `tools` → plugin responds with metadata + tool definitions
4. When the agent calls the tool, tau sends `execute` → plugin responds with result
5. Plugin stays running for subsequent calls (long-lived subprocess)

## Tool Definition

A tool has:
- **name**: Unique identifier (e.g., `my-tool`)
- **description**: LLM-facing description of what the tool does
- **schema**: JSON Schema for the tool's parameters

## Packaging

### `tau.plugin.yaml` — Plugin metadata

```yaml
name: my-plugin
version: 0.1.0
description: My custom tau plugin
author: Your Name
license: MIT
entrypoint: ./my-plugin
```

### Directory structure

```
my-plugin/
├── main.go              # JSON-RPC stdin/stdout server
├── tau.plugin.yaml      # Plugin metadata
├── go.mod               # Go module
├── Makefile             # Build targets
└── README.md            # Usage instructions
```

## Testing

### Manually test via stdin

```bash
echo '{"method":"tools","params":{}}' | ./my-plugin
echo '{"method":"execute","params":{"tool":"my-tool","args":{"input":"test"}}}' | ./my-plugin
```

### Automated test in Go

```go
func TestMyPlugin(t *testing.T) {
    cmd := exec.Command("./my-plugin")
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    cmd.Start()
    defer cmd.Process.Kill()

    // Handshake
    stdin.Write([]byte(`{"method":"tools","params":{}}` + "\n"))
    var resp struct {
        Name  string `json:"name"`
        Tools []struct{ Name string } `json:"tools"`
    }
    json.NewDecoder(stdout).Decode(&resp)
    if resp.Name != "my-plugin" {
        t.Errorf("unexpected name: %s", resp.Name)
    }
}
```

## Examples

### Minimal Plugin (Go)

```go
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
        json.Unmarshal(scanner.Bytes(), &req)
        var resp map[string]any
        switch req.Method {
        case "tools":
            resp = map[string]any{
                "name":    "hello-plugin",
                "version": "0.1.0",
                "tools": []any{
                    map[string]any{
                        "name":        "hello",
                        "description": "Say hello",
                        "schema": map[string]any{
                            "type":       "object",
                            "properties": map[string]any{
                                "name": map[string]any{"type": "string"},
                            },
                        },
                    },
                },
            }
        case "execute":
            args := req.Params["args"].(map[string]any)
            name := args["name"].(string)
            resp = map[string]any{"result": "Hello, " + name + "!"}
        }
        out, _ := json.Marshal(resp)
        fmt.Println(string(out))
    }
}
```

## Best Practices

1. **Stateless**: Don't rely on plugin state between calls. If you need state, use the workspace filesystem.
2. **Fast handshake**: `tools` method should return in <100ms. Don't do expensive init here.
3. **Graceful error**: Always return `{"error":"..."}` on failure, never crash.
4. **Streaming**: For long-running operations, write progress to stderr.
5. **Versioning**: Bump version in `tau.plugin.yaml` when changing tool schemas.
6. **Testing**: Always include a `tools` → `execute` test that runs offline.
7. **Workspace-safe**: Read/write only within the provided workspace root.

## Upgrade Roadmap (v0.1.0 → v0.2.0)

- Add `execute_streaming` method for progressive output
- Add `validate` method for pre-flight validation
- Plugin dependency declaration in `tau.plugin.yaml`

## Reference Implementations

- `cmd/plugin-example/` — Minimal hello-world plugin
- `cmd/plugin-workspace-diag/` — Full-featured workspace diagnostic plugin
- `cmd/plugin-git-diff/` — Git diff plugin
- `cmd/plugin-run-tests/` — Test runner plugin

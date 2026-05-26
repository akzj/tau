# tau Plugin Template

Copy this directory and modify `main.go` to create your own tau plugin.

## Quick Start

```bash
cp -r examples/plugin-template my-plugin
cd my-plugin
# Edit main.go — implement your tool
make build
tau plugin install ./my-plugin
tau plugin list
```

## Files

- `main.go` — JSON-RPC stdin/stdout server
- `tau.plugin.yaml` — Plugin metadata
- `go.mod` — Go module
- `Makefile` — Build targets

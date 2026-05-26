---
name: "shell-scripting"
description: "Write and debug bash scripts safely and idiomatically"
disable-model-invocation: false
---

## Guidelines
- **Safety first**: Always use `set -euo pipefail` at the top of scripts.
- **Quote variables**: Always quote `"$variable"` to prevent word splitting.
- **Use `[[` not `[`**: Bash's `[[` is safer and more featureful than POSIX `[`.
- **Check exit codes**: After critical commands, check `$?` or use `||` for error handling.
- **Temporary files**: Use `mktemp` for temp files, clean up with `trap cleanup EXIT`.
- **Portability**: Prefer `#!/usr/bin/env bash` over `#!/bin/bash`. Avoid bashisms if targeting POSIX.
- **Output**: Use `>&2 echo` for stderr messages. Keep stdout for data output.

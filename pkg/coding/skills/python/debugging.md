---
name: "python-debugging"
description: "Python debugging: pdb, pytest -s, tracebacks, logging"
trigger: "debug|trace|breakpoint python"
language: python
---

## Python Debugging Tools
- **pdb**: `import pdb; pdb.set_trace()` or `breakpoint()` (Python 3.7+). Commands: `n` next, `s` step into, `c` continue, `p` print, `l` list.
- **pytest**: `pytest -s --pdb` — drop into debugger on test failure.
- **Tracebacks**: Read from bottom up — the last line is where the error occurred.
- **Logging**: Use `logging` module, not `print()`. `logging.basicConfig(level=logging.DEBUG)` for verbose output.
- **Interactive exploration**: `python -i script.py` keeps REPL open after execution.

## Process
1. Read the full traceback — find the exact line and error type.
2. Add `breakpoint()` before the failure line.
3. Inspect variables: `p locals()`, `p variable_name`.
4. Trace the call chain backwards until you find the root cause.
---
name: "python-code-review"
description: "Python code review: type hints, GIL, async, exception handling"
trigger: "review|check|audit python code"
language: python
---

## Python-Specific Checks
- **Type hints**: All function signatures must have type annotations. Use `mypy --strict` for enforcement.
- **GIL awareness**: CPU-bound code blocks the GIL. Use `multiprocessing` or offload to C extensions.
- **Async/await**: Don't mix sync and async without `asyncio.run()` or `loop.run_until_complete()`.
- **Exception handling**: Catch specific exceptions, never bare `except:`. Use `finally` for cleanup.
- **Mutable defaults**: Never use `def f(x=[])` — use `None` + check inside function.
- **Virtual environments**: Always specify dependencies in `requirements.txt` or `pyproject.toml`.
- **f-strings**: Prefer over `.format()` or `%` formatting.
- **Context managers**: Use `with` for resource management. Implement `__enter__`/`__exit__`.
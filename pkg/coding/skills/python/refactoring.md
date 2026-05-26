---
name: "python-refactoring"
description: "Python refactoring: dataclasses, generators, comprehensions, typing"
trigger: "refactor|clean|simplify python"
language: python
---

## Python Refactoring Patterns
- **dataclass migration**: Replace manual `__init__` with `@dataclass` for data containers.
- **Comprehensions**: Replace `map`/`filter` + `lambda` with list/dict/set comprehensions.
- **Generators**: Use `yield` for lazy evaluation of large sequences. Replace `return []` with `yield`.
- **Pathlib**: Replace `os.path.*` with `pathlib.Path` for cleaner path manipulation.
- **Walrus operator**: Use `:=` for assignment expressions in `while`/`if` (Python 3.8+).
- **Type narrowing**: Use `assert isinstance(x, Type)` or `if isinstance(...)` for mypy type narrowing.
- **Exception groups**: Use `except*` for concurrent exception handling (Python 3.11+).

## Process
1. Run `mypy --strict` to establish type baseline.
2. Make one refactoring change.
3. Run tests + mypy after each change.
4. Commit when green.
---
name: "python-test-writing"
description: "Python test writing: pytest fixtures, parametrize, mocking, coverage"
trigger: "test|cover|mock python|pytest"
language: python
---

## Python Test Patterns
- **pytest fixtures**: `@pytest.fixture` + `conftest.py` for shared setup. Use `yield` for teardown.
- **Parametrize**: `@pytest.mark.parametrize("input,expected", [(1,2),(3,4)])` — one test, many cases.
- **Mocking**: `unittest.mock.patch` for external dependencies. Mock at the boundary, not internals.
- **Coverage**: `pytest --cov=. --cov-report=html` for coverage reports.
- **Temporary files**: `tmp_path` fixture for isolated file I/O tests.
- **Fixtures vs. helpers**: Fixtures for setup/teardown, plain functions for data generation.

## Process
1. Run existing tests: `pytest -v` — baseline.
2. Write parametrized test covering all code paths.
3. Mock external calls (APIs, databases, filesystem).
4. Run `pytest --cov` to verify coverage.
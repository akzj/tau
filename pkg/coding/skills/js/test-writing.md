---
name: "js-test-writing"
description: "JS/TS test writing: Jest, Vitest, mocking, snapshots, coverage"
trigger: "test|cover|mock js|jest|vitest|typescript"
language: js
---

## JS/TS Test Patterns
- **Jest**: `describe`/`it` blocks. `expect(value).toBe(expected)` or `.toEqual()` for objects.
- **Vitest**: Compatible with Jest API. Faster, ESM-native. Use for new projects.
- **Mocking**: `jest.mock('./module')` for module-level mocks. `jest.fn()` for function spies.
- **Snapshot testing**: `expect(component).toMatchSnapshot()` — great for UI regression.
- **Coverage**: `jest --coverage` or `vitest --coverage`.
- **Testing async**: Use `async/await` in tests. Jest/vitest handles Promise resolution.
- **Setup/teardown**: `beforeEach`/`afterEach` for per-test setup. `beforeAll`/`afterAll` for suite-level.

## Process
1. Write test for the bug or feature.
2. Cover: happy path, error path, edge cases (null, undefined, empty, large input).
3. Run `npx jest --coverage` to verify.
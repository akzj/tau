---
name: "js-refactoring"
description: "JS/TS refactoring: Promise→async, arrow functions, destructuring, modules"
trigger: "refactor|clean|simplify js|javascript|typescript"
language: js
---

## JS/TS Refactoring Patterns
- **Promise→async/await**: Replace `.then().catch()` chains with `try { await ... } catch { ... }`.
- **Arrow functions**: Use arrow functions for callbacks and short expressions. Keep `function` for methods needing `this`.
- **Destructuring**: `const { a, b } = obj` instead of `const a = obj.a`.
- **Template literals**: Use backtick strings with `${var}` instead of `+` concatenation.
- **Optional chaining**: Replace `obj && obj.prop && obj.prop.val` with `obj?.prop?.val`.
- **Nullish coalescing**: Replace `value || default` with `value ?? default` (only falls back on null/undefined).
- **ES modules**: Use `import`/`export` consistently. Avoid mixing with `require`.
- **TypeScript strict**: Enable `strict: true` in `tsconfig.json`. Add types incrementally.

## Process
1. Run tests: `npm test` — establish baseline.
2. Make one refactoring change.
3. Run tests after each change.
4. Run `npx tsc --noEmit` for TypeScript type checking.
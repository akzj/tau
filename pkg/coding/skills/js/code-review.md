---
name: "js-code-review"
description: "JavaScript/TypeScript code review: async/await, promises, types, security"
trigger: "review|check|audit js|javascript|typescript|ts code"
language: js
---

## JS/TS-Specific Checks
- **Async/await**: Every `await` must be inside `async`. Don't mix `.then()` and `await`.
- **Promise handling**: Never leave a Promise floating (unhandled rejection). Always `.catch()` or `await` in `try`.
- **Type safety (TS)**: No `any` in production code. Use `unknown` + type guards instead.
- **Null checks**: Use optional chaining `?.` and nullish coalescing `??`.
- **Memory leaks**: Remove event listeners in cleanup. Clear intervals/timeouts on unmount.
- **XSS prevention**: Never use `innerHTML` with user data. Use `textContent` or sanitize.
- **Dependency freshness**: Check for known vulnerabilities: `npm audit`.
- **Bundle size**: Don't import entire libraries for one function.
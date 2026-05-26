---
name: "js-debugging"
description: "JS/TS debugging: Node inspector, Chrome DevTools, console methods"
trigger: "debug|inspect|breakpoint js|javascript|typescript|node"
language: js
---

## JS/TS Debugging Tools
- **Node inspector**: `node --inspect-brk script.js` → open `chrome://inspect` in Chrome.
- **Chrome DevTools**: Breakpoints, watch expressions, call stack, network tab.
- **console methods**: `console.table()` for arrays, `console.group()` for nesting, `console.time()`/`console.timeEnd()` for profiling.
- **debugger statement**: `debugger;` inline breakpoint (remove before commit).
- **Source maps**: Enable in build config for readable stack traces in production.

## Process
1. Reproduce the bug. Note the exact error message and stack trace.
2. Set a breakpoint at the line above the error.
3. Step through, inspecting variables at each step.
4. Identify the first point where state diverges from expected.
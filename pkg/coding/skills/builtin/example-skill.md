---
name: "code-review"
description: "Review code for bugs, style issues, and improvements"
disable-model-invocation: false
---

## Guidelines
- Check for unhandled errors, nil dereferences, race conditions.
- Verify that function signatures match their call sites.
- Look for missing imports, unused variables, shadowed names.
- Suggest idiomatic Go patterns where applicable.
- Be concise: report issues, don't rewrite the whole file.
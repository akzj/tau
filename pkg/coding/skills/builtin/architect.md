---
name: "architect"
description: "Architecture review: module boundaries, dependency direction, interface design, coupling"
disable-model-invocation: false
trigger: "architecture|design|boundary|interface|dependency|coupling|module"
tools: [read, grep, glob]
---

## Dimensions
- **Module boundaries**: Are packages well-defined with clear responsibilities? Does each package have a single purpose?
- **Dependency direction**: Do dependencies flow inward (details → abstractions)? Are there circular dependencies?
- **Interface design**: Are interfaces small (1-3 methods)? Defined where consumed, not where implemented?
- **Coupling**: Can you change one module without affecting others? Are there hidden dependencies (global state, init functions)?
- **Cohesion**: Do related things live together? Are unrelated things separated?

## Process
1. Map the module graph: `grep` for imports across packages.
2. Identify dependency violations: core importing product packages, circular imports.
3. Check interface sizes: `grep "type.*interface"` across packages.
4. Report findings: violations, risks, improvement suggestions.
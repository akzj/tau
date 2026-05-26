---
name: "documentation-generation"
description: "Documentation: godoc comments, README, ADR, API docs, diagrams-as-code"
disable-model-invocation: false
trigger: "document|docstring|godoc|readme|ADR|api docs|diagram"
tools: [read, write, edit, grep, glob, bash]
---

## Types
- **Godoc**: Every exported symbol must have a one-line comment: `// Name does X.` Run `go doc` to verify.
- **README**: Project overview, quick start, architecture diagram, links to docs.
- **ADR (Architecture Decision Record)**: Document why a decision was made. Title, context, decision, consequences.
- **API docs**: OpenAPI/Swagger for REST APIs. Request/response schemas, error codes, authentication.
- **Diagrams as code**: Use Mermaid or PlantUML. Embed in Markdown. Version controlled with code.

## Steps
1. Add godoc to all exported symbols: `grep -L "// " *.go` to find undocumented exports.
2. Write README with: project name, one-liner, quick start, architecture, links.
3. Create ADR template: `docs/adr/001-title.md`. Fill in context, decision, consequences.
4. Document API endpoints with request/response examples and error codes.
5. Add architecture diagram: ASCII art or Mermaid. Keep it with the code.

---
name: "cicd-patterns"
description: "CI/CD design: pipelines, canary deployments, blue-green, rollback, health checks"
disable-model-invocation: false
trigger: "ci|cd|pipeline|deploy|rollback|canary|blue-green"
tools: [read, write, edit, bash, grep, glob]
---

## Patterns
- **Pipeline stages**: Lint → Test → Build → Deploy (staging) → Verify → Deploy (production). Fail fast.
- **Canary deployment**: Deploy to 5% of users → monitor metrics → increase to 100%. Rollback on error spike.
- **Blue-Green**: Two identical environments. Deploy to inactive, switch traffic, keep old for rollback.
- **Rollback**: Every deployment must be reversible. Database migrations must have down scripts.
- **Health checks**: `/health` (liveness) + `/ready` (readiness). Grace period for startup.

## Steps
1. Define pipeline in CI config (`.github/workflows/` or similar).
2. Add lint stage first (fastest feedback).
3. Add test stage with race detector: `go test -race ./...`
4. Add build stage with version ldflags.
5. Configure deployment with health check verification and automatic rollback on failure.

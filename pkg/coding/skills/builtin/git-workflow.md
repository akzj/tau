---
name: "git-workflow"
description: "Git commands and workflows: branching, committing, rebasing, PR management"
disable-model-invocation: false
---

## Guidelines
- **Branch naming**: feature/<name>, fix/<name>, refactor/<name>. Lowercase, hyphen-separated.
- **Commits**: Atomic, focused. One logical change per commit. Write in imperative mood ("Add X", "Fix Y").
- **Before committing**: Run tests and build. Never commit broken code.
- **Rebase, don't merge**: Keep history linear. `git pull --rebase` before pushing.
- **PR descriptions**: What changed, why, how to test. Link related issues.
- **Check status often**: `git status` before and after operations.
- **Stash before switching**: If you have uncommitted changes, stash or commit before changing branches.
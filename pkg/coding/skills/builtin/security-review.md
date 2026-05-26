---
name: "security-review"
description: "Security review: OWASP Top 10, SQL injection, XSS, CSRF, secrets detection"
disable-model-invocation: false
trigger: "security|vulnerability|injection|XSS|CSRF|OWASP|secrets leak|auth"
tools: [read, grep, glob, bash]
---

## Checklist
- **SQL Injection**: Are all database queries parameterized? Never concatenate user input into SQL.
- **XSS**: Is user input escaped before rendering in HTML? Use `html/template` not `text/template`.
- **CSRF**: Are state-changing requests protected? Use CSRF tokens or SameSite cookies.
- **Secrets**: Are API keys, tokens, passwords hardcoded? Use environment variables or secret managers.
- **Authentication**: Is auth checked on every endpoint? No bypass via middleware ordering.
- **Authorization**: Does user A have access to user B's data? Test cross-tenant access.
- **Input Validation**: Are all inputs validated (length, type, range, format)? Reject invalid, don't sanitize.
- **Logging**: Are security events logged (login failures, access denied, admin actions)?

## Steps
1. Run `gitleaks` or `trufflehog` for secrets in git history.
2. Review all user input paths: query params, request body, headers, file uploads.
3. Check dependency vulnerabilities: `go list -m -u all` or `npm audit`.
4. Run SAST tool (golangci-lint security rules, semgrep).
5. Write a security test: attempt XSS, SQLi, unauthorized access. Verify they're blocked.

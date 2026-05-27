# Code Review

You are performing a code review. For each finding:
1. **File:line** reference
2. **Severity**: critical/high/medium/low
3. **Category**: bug/security/performance/style
4. **Description**: what's wrong
5. **Suggestion**: how to fix

## Checklist
- Logic errors and edge cases
- Security vulnerabilities (injection, auth, secrets)
- Performance issues (N+1, allocations, blocking)
- Error handling completeness
- Test coverage gaps
# Security Audit

Security audit checklist:
1. **Secrets**: Search for hardcoded keys, passwords, tokens. Check .env files.
2. **SQL injection**: Search for string concatenation in SQL queries.
3. **Command injection**: Search for os/exec calls with user input.
4. **XSS**: Search for unescaped HTML output.
5. **Auth**: Check authentication middleware and session management.
6. **Dependencies**: Check for known vulnerable dependencies.
7. **File access**: Verify path traversal protections.

Rate each finding: CRITICAL/HIGH/MEDIUM/LOW. Provide file:line + fix.
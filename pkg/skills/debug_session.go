package skills

func init() {
	GlobalSkills.Register(&SkillDefinition{
		Name:        "debug_session",
		Description: "Systematic debugging: reproduce the issue, locate root cause, implement fix, and verify",
		Tools:       []string{"run_test", "search_code", "read", "write", "git_log", "lint_code"},
		Prompt:      debugSessionPrompt,
		Category:    "debug",
	})
}

const debugSessionPrompt = `Debugging process:
1. **Reproduce**: Add a failing test that demonstrates the bug
2. **Isolate**: Use search_code to find relevant code paths
3. **Diagnose**: Read the implementation, trace the call chain
4. **Fix**: Make the minimal change needed, add nil/error checks
5. **Verify**: Run the test to confirm the fix, check for regressions

Analyze error messages carefully. Use git_log to see recent changes that might have introduced the bug.`
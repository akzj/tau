package skills

func init() {
	GlobalSkills.Register(&SkillDefinition{
		Name:        "refactor_module",
		Description: "Refactor code safely: lock in tests, refactor, verify all tests pass",
		Tools:       []string{"run_test", "find_references", "format_code", "search_code", "write", "read"},
		Prompt:      refactorModulePrompt,
		Category:    "refactor",
	})
}

const refactorModulePrompt = `Safe refactoring protocol:
1. **Test-lock**: Run all existing tests. If any fail, fix BEFORE refactoring.
2. **Find references**: Use find_references to see who depends on the code you're refactoring.
3. **Plan**: Identify what to extract, rename, or simplify.
4. **Execute**: One change at a time. Run tests after each step.
5. **Format**: Use format_code after each change.
6. **Verify**: Full test suite must pass. No behavior change allowed.

If tests fail at any step, revert and fix before continuing.`
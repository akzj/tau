package skills

func init() {
	GlobalSkills.Register(&SkillDefinition{
		Name:        "onboard_newcomer",
		Description: "Help a new developer understand the project: structure, key modules, and how to make changes",
		Tools:       []string{"read", "search_code", "find_references", "browse", "git_log"},
		Prompt:      onboardNewcomerPrompt,
		Category:    "onboarding",
	})
}

const onboardNewcomerPrompt = `Project onboarding guide:
1. **Overview**: Read README and main module. Summarize project purpose.
2. **Structure**: Map the directory tree. Identify key modules and their roles.
3. **Entry points**: Find main(), key interfaces, and configuration.
4. **Workflows**: Show how common operations work (e.g., "how to add a new endpoint", "how to modify the build").
5. **Testing**: Explain test structure and how to run tests.
6. **Common changes**: Based on git_log, identify frequently changed areas.

Present as a structured tour with code snippets and file references.`
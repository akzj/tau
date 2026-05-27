// Package skills provides domain skill definitions and registry for agent workflows.
// Each skill prescribes a sequence of tools and prompt guidance for a specific task domain.
package skills

import (
	"fmt"
	"sort"
	"strings"
)

// SkillDefinition describes a domain skill with prescribed tools and prompt.
type SkillDefinition struct {
	Name        string   // unique skill identifier
	Description string   // human-readable summary
	Tools       []string // prescribed tool names in recommended order
	Prompt      string   // loaded prompt template
	Category    string   // grouping: quality, debug, refactor, onboarding, security, performance
}

// SkillRegistry manages registered domain skills.
type SkillRegistry struct {
	skills map[string]*SkillDefinition
}

// GlobalSkills is the default skill registry populated via init() in each skill file.
var GlobalSkills = NewSkillRegistry()

// NewSkillRegistry creates an empty skill registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{skills: make(map[string]*SkillDefinition)}
}

// Register adds a skill definition to the registry.
func (r *SkillRegistry) Register(skill *SkillDefinition) {
	r.skills[skill.Name] = skill
}

// Get retrieves a skill by name.
func (r *SkillRegistry) Get(name string) (*SkillDefinition, bool) {
	s, ok := r.skills[name]
	return s, ok
}

// List returns all registered skills sorted by name.
func (r *SkillRegistry) List() []*SkillDefinition {
	var names []string
	for n := range r.skills {
		names = append(names, n)
	}
	sort.Strings(names)
	var result []*SkillDefinition
	for _, n := range names {
		result = append(result, r.skills[n])
	}
	return result
}

// Count returns the number of registered skills.
func (r *SkillRegistry) Count() int {
	return len(r.skills)
}

// BuildPrompt constructs a system prompt injection for a skill with optional context.
func (r *SkillRegistry) BuildPrompt(name string, context string) (string, error) {
	skill, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Skill: %s\n", skill.Name))
	b.WriteString(fmt.Sprintf("%s\n\n", skill.Description))
	b.WriteString("### Recommended Tool Sequence:\n")
	for i, t := range skill.Tools {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, t))
	}
	if context != "" {
		b.WriteString(fmt.Sprintf("\n### Context:\n%s\n", context))
	}
	b.WriteString(fmt.Sprintf("\n%s\n", skill.Prompt))
	return b.String(), nil
}
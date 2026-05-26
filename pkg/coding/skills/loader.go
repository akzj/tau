package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Skill represents a loaded skill.
type Skill struct {
	Name              string
	Description       string
	Content           string
	SourcePath        string
	DisableInvocation bool
}

// Loader discovers and loads skills from configured directories.
type Loader struct {
	dirs []string
}

// NewLoader creates a skill loader for the given directories.
// Dirs are searched in order; later dirs override earlier by name.
func NewLoader(dirs []string) *Loader {
	return &Loader{dirs: dirs}
}

// Load discovers all skills across configured directories.
func (l *Loader) Load() ([]Skill, error) {
	seen := make(map[string]Skill)
	for _, dir := range l.dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skills: cannot read %s: %v\n", dir, err)
			continue // skip missing directories
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skills: cannot read %s: %v\n", path, err)
				continue
			}
			skill := parseSkillFile(string(content), path)
			if skill.Name == "" {
				fmt.Fprintf(os.Stderr, "skills: skipping %s (no name)\n", path)
				continue
			}
			seen[skill.Name] = skill
		}
	}
	var result []Skill
	for _, s := range seen {
		result = append(result, s)
	}
	return result, nil
}

// parseSkillFile extracts frontmatter + content from SKILL.md files.
// Format:
//
//	---
//	name: "rust-refactor"
//	description: "Rust refactoring"
//	disable-model-invocation: false
//	---
//	## Guidelines
//	...
func parseSkillFile(content, path string) Skill {
	s := Skill{SourcePath: path}

	// Try frontmatter parsing
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) >= 3 {
		frontmatter := parts[1]
		s.Content = parts[2]
		for _, line := range strings.Split(frontmatter, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "name:") {
				s.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				s.Name = strings.Trim(s.Name, "\"")
			}
			if strings.HasPrefix(line, "description:") {
				s.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				s.Description = strings.Trim(s.Description, "\"")
			}
			if strings.HasPrefix(line, "disable-model-invocation:") {
				val := strings.TrimSpace(strings.TrimPrefix(line, "disable-model-invocation:"))
				s.DisableInvocation = val == "true"
			}
		}
	} else {
		s.Content = content
	}

	// Fallback: derive name from filename
	if s.Name == "" {
		base := filepath.Base(path)
		s.Name = strings.TrimSuffix(base, ".md")
	}
	if s.Description == "" {
		s.Description = s.Name
	}

	return s
}
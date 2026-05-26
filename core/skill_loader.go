package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillInfo holds a loaded skill's metadata.
type SkillInfo struct {
	Name        string
	Description string
	Content     string
	Path        string
	Category    string
	Keywords    []string // derived from name+description
}

// SkillLoader loads and matches skills from directories.
type SkillLoader struct {
	skills []SkillInfo
}

// NewSkillLoader scans skill directories and loads all skills.
func NewSkillLoader(dirs ...string) *SkillLoader {
	sl := &SkillLoader{}
	for _, dir := range dirs {
		sl.scanDir(dir)
	}
	return sl
}

func (sl *SkillLoader) scanDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		skill := parseSkill(string(data), path)
		sl.skills = append(sl.skills, skill)
	}
}

func parseSkill(raw, path string) SkillInfo {
	s := SkillInfo{Path: path}
	lines := strings.Split(raw, "\n")
	inFrontmatter := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if !inFrontmatter {
			s.Content += line + "\n"
			continue
		}
		if strings.HasPrefix(line, "name:") {
			s.Name = strings.Trim(strings.TrimPrefix(line, "name:"), " \"")
		}
		if strings.HasPrefix(line, "description:") {
			s.Description = strings.Trim(strings.TrimPrefix(line, "description:"), " \"")
		}
		if strings.HasPrefix(line, "when_to_use:") {
			s.Category = strings.Trim(strings.TrimPrefix(line, "when_to_use:"), " \"")
		}
	}
	text := strings.ToLower(s.Name + " " + s.Description + " " + s.Category)
	s.Keywords = strings.Fields(text)
	if s.Name == "" {
		s.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	if s.Description == "" {
		s.Description = s.Name
	}
	s.Content = strings.TrimSpace(s.Content)
	return s
}

// MatchSkills finds skills relevant to the given context.
func (sl *SkillLoader) MatchSkills(toolNames []string, taskDesc string, maxResults, maxChars int) []SkillInfo {
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxChars <= 0 {
		maxChars = 200
	}

	context := strings.ToLower(strings.Join(toolNames, " ") + " " + taskDesc)
	contextWords := strings.Fields(context)

	type scored struct {
		skill SkillInfo
		score int
	}
	var ranked []scored
	seen := make(map[string]bool)

	for _, sk := range sl.skills {
		if seen[sk.Name] {
			continue
		}
		seen[sk.Name] = true

		score := 0
		for _, kw := range sk.Keywords {
			for _, cw := range contextWords {
				if kw == cw {
					score += 2
				}
				if strings.Contains(kw, cw) || strings.Contains(cw, kw) {
					score++
				}
			}
		}
		if score > 0 {
			ranked = append(ranked, scored{sk, score})
		}
	}

	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	var result []SkillInfo
	for i := 0; i < len(ranked) && i < maxResults; i++ {
		sk := ranked[i].skill
		if len(sk.Content) > maxChars {
			sk.Content = sk.Content[:maxChars] + "..."
		}
		result = append(result, sk)
	}
	return result
}

// FormatSkills formats matched skills for injection into system prompt.
func FormatSkills(skills []SkillInfo) string {
	if len(skills) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, "## Relevant Skills")
	for _, sk := range skills {
		lines = append(lines, fmt.Sprintf("- **%s**: %s", sk.Name, sk.Description))
	}
	return strings.Join(lines, "\n")
}

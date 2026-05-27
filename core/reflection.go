package core

import (
	"fmt"
	"regexp"
	"strings"
)

// ReflectionIssue represents a detected problem in agent output.
type ReflectionIssue struct {
	Type        string // "syntax", "logic", "incomplete", "hallucination"
	Severity    string // "error", "warn"
	Description string
	Location    string // e.g., "line 5", "code block"
	Suggestion  string // correction hint
}

// ReflectionResult holds the outcome of a reflection pass.
type ReflectionResult struct {
	Passed    bool
	Issues    []ReflectionIssue
	Rounds    int
	Corrected bool
}

// ReflectionEngine reviews agent output and detects issues.
// Acts as a post-processing hook — does not block the agent loop.
type ReflectionEngine struct {
	MaxRounds int // max correction retries (default 2)
	MaxIssues int // max issues to report per round (default 10)
}

// NewReflectionEngine creates a reflection engine.
func NewReflectionEngine(maxRounds int) *ReflectionEngine {
	if maxRounds <= 0 {
		maxRounds = 2
	}
	return &ReflectionEngine{MaxRounds: maxRounds, MaxIssues: 10}
}

// Review examines agent output and returns detected issues.
// Categories: syntax, logic, incomplete, hallucination.
func (re *ReflectionEngine) Review(output string) ReflectionResult {
	result := ReflectionResult{Passed: true}
	if re.MaxIssues <= 0 {
		re.MaxIssues = 10
	}

	// Rule 1: Syntax errors in code blocks
	re.detectSyntax(output, &result)

	// Rule 2: Logic contradictions (e.g., "this works" vs "this fails")
	re.detectLogicContradictions(output, &result)

	// Rule 3: Incomplete tasks (phrases like "TODO", "not yet", "pending")
	re.detectIncomplete(output, &result)

	// Rule 4: Hallucination (references to non-existent files / common patterns)
	re.detectHallucination(output, &result)

	result.Passed = len(result.Issues) == 0
	return result
}

// BuildCorrectionPrompt creates a prompt to fix detected issues.
func (re *ReflectionEngine) BuildCorrectionPrompt(output string, issues []ReflectionIssue) string {
	var b strings.Builder
	b.WriteString("## Reflection found issues in your output:\n\n")
	for i, issue := range issues {
		if i >= re.MaxIssues {
			break
		}
		b.WriteString(fmt.Sprintf("%d. **[%s] %s**\n", i+1, issue.Type, issue.Description))
		b.WriteString(fmt.Sprintf("   Location: %s\n", issue.Location))
		b.WriteString(fmt.Sprintf("   Suggestion: %s\n\n", issue.Suggestion))
	}
	b.WriteString("Please correct these issues and provide the corrected output.")
	return b.String()
}

// --- Detection Rules ---

func (re *ReflectionEngine) detectSyntax(output string, result *ReflectionResult) {
	if len(result.Issues) >= re.MaxIssues {
		return
	}

	codeBlocks := extractCodeBlocks(output)
	for _, block := range codeBlocks {
		if len(result.Issues) >= re.MaxIssues {
			break
		}
		if unbalancedBraces(block.content) {
			result.Issues = append(result.Issues, ReflectionIssue{
				Type:        "syntax",
				Severity:    "error",
				Description: "Unmatched braces/parentheses in code block",
				Location:    block.language,
				Suggestion:  "Check brace/parenthesis matching",
			})
		}
		if missingReturn(block.content) {
			result.Issues = append(result.Issues, ReflectionIssue{
				Type:        "syntax",
				Severity:    "error",
				Description: "Function missing return statement",
				Location:    block.language,
				Suggestion:  "Add return statement to function",
			})
		}
		if duplicateDeclaration(block.content) {
			result.Issues = append(result.Issues, ReflectionIssue{
				Type:        "syntax",
				Severity:    "error",
				Description: "Duplicate variable/function declaration",
				Location:    block.language,
				Suggestion:  "Remove duplicate or rename variable",
			})
		}
	}
}

func (re *ReflectionEngine) detectLogicContradictions(output string, result *ReflectionResult) {
	if len(result.Issues) >= re.MaxIssues {
		return
	}

	lower := strings.ToLower(output)
	if strings.Contains(lower, "works") && strings.Contains(lower, "error") {
		result.Issues = append(result.Issues, ReflectionIssue{
			Type:        "logic",
			Severity:    "warn",
			Description: "Output contains both success and error claims — possible contradiction",
			Location:    "full response",
			Suggestion:  "Clarify whether the code works or has errors",
		})
	}
}

func (re *ReflectionEngine) detectIncomplete(output string, result *ReflectionResult) {
	if len(result.Issues) >= re.MaxIssues {
		return
	}

	markers := []string{"TODO", "FIXME", "not yet implemented", "pending", "to be done", "work in progress"}
	lower := strings.ToLower(output)
	for _, marker := range markers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			result.Issues = append(result.Issues, ReflectionIssue{
				Type:        "incomplete",
				Severity:    "warn",
				Description: fmt.Sprintf("Output contains incomplete marker: %q", marker),
				Location:    "full response",
				Suggestion:  "Complete the implementation or remove the marker",
			})
			break
		}
	}
}

func (re *ReflectionEngine) detectHallucination(output string, result *ReflectionResult) {
	if len(result.Issues) >= re.MaxIssues {
		return
	}

	patterns := []string{
		`the file .* exists`,
		`the function .* does`,
		`the package .* provides`,
	}
	for _, pat := range patterns {
		re := regexp.MustCompile(`(?i)` + pat)
		if re.MatchString(output) {
			result.Issues = append(result.Issues, ReflectionIssue{
				Type:        "hallucination",
				Severity:    "warn",
				Description: "Claim about file/function existence without verification",
				Location:    "full response",
				Suggestion:  "Use tool to verify file/function existence before claiming",
			})
			break
		}
	}
}

// --- Helpers ---

type codeBlock struct {
	language string
	content  string
}

func extractCodeBlocks(text string) []codeBlock {
	re := regexp.MustCompile("```(\\w*)\\n([\\s\\S]*?)\\n```")
	matches := re.FindAllStringSubmatch(text, -1)
	var blocks []codeBlock
	for _, m := range matches {
		if len(m) >= 3 {
			blocks = append(blocks, codeBlock{language: m[1], content: m[2]})
		}
	}
	return blocks
}

func unbalancedBraces(code string) bool {
	stack := 0
	for _, r := range code {
		if r == '{' || r == '(' {
			stack++
		}
		if r == '}' || r == ')' {
			stack--
		}
	}
	return stack != 0
}

func missingReturn(code string) bool {
	lower := strings.ToLower(code)
	hasFunc := strings.Contains(lower, "func ") && strings.Contains(lower, ") ")
	hasReturn := strings.Contains(lower, "return ")
	hasReturnType := strings.Contains(lower, "int") ||
		strings.Contains(lower, "string") ||
		strings.Contains(lower, "error")
	return hasFunc && !hasReturn && hasReturnType
}

func duplicateDeclaration(code string) bool {
	lines := strings.Split(code, "\n")
	seen := make(map[string]bool)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "var ") || strings.HasPrefix(line, "func ") {
			if seen[line] {
				return true
			}
			seen[line] = true
		}
	}
	return false
}

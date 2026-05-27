package core

import (
	"context"
	"fmt"
	"strings"
)

// KnowledgeExtractor extracts semantic facts from interaction records.
type KnowledgeExtractor interface {
	Extract(ctx context.Context, episode Episode) ([]SemanticFact, error)
}

// SimpleExtractor is a rule-based knowledge extractor (no LLM required).
type SimpleExtractor struct{}

// Extract extracts 1-3 facts from an episode using simple heuristics.
func (se *SimpleExtractor) Extract(ctx context.Context, ep Episode) ([]SemanticFact, error) {
	var facts []SemanticFact
	confidence := 0.7

	// Rule 1: Error patterns → "avoid X in Y context"
	if strings.Contains(strings.ToLower(ep.Trigger), "error") {
		fact := SemanticFact{
			Content:       fmt.Sprintf("Avoid %s — %s", extractKeyPhrase(ep.Trigger), ep.Lesson),
			SourceEpisode: ep.ID,
			Confidence:    confidence,
			Tags:          append(ep.Tags, "error-pattern"),
		}
		facts = append(facts, fact)
	}

	// Rule 2: Success patterns → "use X for Y"
	if strings.Contains(strings.ToLower(ep.Outcome), "resolved") || strings.Contains(strings.ToLower(ep.Outcome), "success") {
		fact := SemanticFact{
			Content:       fmt.Sprintf("Use %s — effective for %s", extractKeyPhrase(ep.Action), extractKeyPhrase(ep.Trigger)),
			SourceEpisode: ep.ID,
			Confidence:    confidence + 0.1,
			Tags:          append(ep.Tags, "success-pattern"),
		}
		facts = append(facts, fact)
	}

	// Rule 3: Generic lesson extraction
	if ep.Lesson != "" && ep.Lesson != "error during operation" && ep.Lesson != "error during tool execution" {
		fact := SemanticFact{
			Content:       fmt.Sprintf("Lesson: %s", ep.Lesson),
			SourceEpisode: ep.ID,
			Confidence:    0.6,
			Tags:          append(ep.Tags, "lesson"),
		}
		facts = append(facts, fact)
	}

	if len(facts) == 0 {
		// Default: capture the trigger-action-outcome
		facts = append(facts, SemanticFact{
			Content:       fmt.Sprintf("When %s, tried %s → %s", truncateStr(ep.Trigger, 40), truncateStr(ep.Action, 30), truncateStr(ep.Outcome, 30)),
			SourceEpisode: ep.ID,
			Confidence:    0.4,
			Tags:          ep.Tags,
		})
	}

	return facts, nil
}

// extractKeyPhrase extracts a short key phrase from text.
func extractKeyPhrase(s string) string {
	s = strings.TrimPrefix(s, "error: ")
	s = strings.TrimPrefix(s, "Error: ")
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return s
}

// truncateStr truncates a string to max length.
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

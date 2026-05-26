package core

import "strings"

// CompactionConfig holds settings for automatic compaction.
type CompactionConfig struct {
	TokenThreshold int // trigger when estimated tokens exceed this
	KeepRecent     int // keep this many recent messages after compaction
}

// DefaultCompactionConfig returns sensible defaults.
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		TokenThreshold: 100000, // 100K tokens
		KeepRecent:     10,
	}
}

// EstimateTokens returns a rough token count.
// Approximation: 4 characters ≈ 1 token.
func EstimateTokens(msgs []Message) int {
	chars := 0
	for _, m := range msgs {
		chars += len(m.Content)
		for _, tc := range m.ToolCalls {
			chars += len(tc.Args) + len(tc.ToolName) + len(tc.CallID)
		}
	}
	return chars / 4
}

// MaybeCompact checks if compaction is needed and triggers the BeforeCompaction hook.
// Returns true if compaction was performed.
func MaybeCompact(sess *Session, cfg CompactionConfig) bool {
	msgs := sess.Transcript.Messages()
	if EstimateTokens(msgs) < cfg.TokenThreshold {
		return false
	}
	if len(msgs) <= cfg.KeepRecent {
		return false
	}

	firstKept := len(msgs) - cfg.KeepRecent

	// Extract file ops for compaction context
	var fileOps []string
	seen := make(map[string]bool)
	for _, m := range msgs[:firstKept] {
		for _, tc := range m.ToolCalls {
			path := extractPath(tc.Args)
			if path == "" {
				continue
			}
			key := tc.ToolName + ":" + path
			if !seen[key] {
				seen[key] = true
				fileOps = append(fileOps, key)
			}
		}
	}

	req := CompactionRequest{
		Summary:          "",
		FirstKeptEntryID: Position(firstKept),
		TokensBefore:     EstimateTokens(msgs[:firstKept]),
		FileOpsHint:      fileOps,
	}

	Logger().Info("compaction: triggered", "tokensBefore", req.TokensBefore)

	// Run the hook — product-layer handler fills in req.Summary via Provider.Complete()
	result, err := sess.Hooks.BeforeCompaction.Run(sess.Context(), req)
	if err != nil || result.Summary == "" {
		return false
	}

	// Replace truncated messages with summary
	sess.Transcript.Compact(result.Summary, Position(firstKept))
	sess.Summary = result.Summary
	return true
}

// extractPath extracts a file path from tool call args JSON.
func extractPath(argsJSON string) string {
	for _, key := range []string{`"file_path"`, `"path"`} {
		idx := strings.Index(argsJSON, key)
		if idx >= 0 {
			rest := argsJSON[idx+len(key):]
			// Skip :" or : "
			rest = strings.TrimLeft(rest, `: "`)
			end := strings.IndexAny(rest, `",}`)
			if end > 0 {
				return rest[:end]
			}
		}
	}
	return ""
}

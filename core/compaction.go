package core

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

	req := CompactionRequest{
		Summary:          "",
		FirstKeptEntryID: Position(firstKept),
		TokensBefore:     EstimateTokens(msgs[:firstKept]),
	}

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

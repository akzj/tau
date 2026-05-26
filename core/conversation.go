package core

// ContextStrategy defines how messages are managed when the token budget is exceeded.
type ContextStrategy string

const (
	// StrategySliding keeps the most recent messages (default).
	StrategySliding ContextStrategy = "sliding"
	// StrategyTruncate keeps the earliest messages, dropping newer ones.
	StrategyTruncate ContextStrategy = "truncate"
	// StrategySummarize uses LLM to summarize old messages, keeping recent ones.
	StrategySummarize ContextStrategy = "summarize"
)

// Conversation manages a bounded message list with token budget awareness.
type Conversation struct {
	Messages  []Message
	MaxTokens int
	Strategy  ContextStrategy
}

// NewConversation creates a Conversation with the given strategy and token limit.
func NewConversation(maxTokens int, strategy ContextStrategy) *Conversation {
	if maxTokens <= 0 {
		maxTokens = 128000 // 128k default
	}
	if strategy == "" {
		strategy = StrategySliding
	}
	return &Conversation{
		MaxTokens: maxTokens,
		Strategy:  strategy,
	}
}

// Add appends messages and trims to fit the token budget.
func (c *Conversation) Add(msgs ...Message) {
	c.Messages = append(c.Messages, msgs...)
	c.FitToWindow()
}

// FitToWindow trims messages to stay within MaxTokens using the configured Strategy.
func (c *Conversation) FitToWindow() {
	if len(c.Messages) == 0 {
		return
	}
	tokens := EstimateTokens(c.Messages)
	if tokens <= c.MaxTokens {
		return
	}

	switch c.Strategy {
	case StrategySliding:
		// Keep most recent messages
		for len(c.Messages) > 2 && EstimateTokens(c.Messages) > c.MaxTokens {
			c.Messages = c.Messages[1:]
		}
	case StrategyTruncate:
		// Keep earliest messages
		for len(c.Messages) > 2 && EstimateTokens(c.Messages) > c.MaxTokens {
			c.Messages = c.Messages[:len(c.Messages)-1]
		}
		// StrategySummarize handled by summarizer.go
	}
}

// ToMessages returns a snapshot copy of the conversation messages.
func (c *Conversation) ToMessages() []Message {
	result := make([]Message, len(c.Messages))
	copy(result, c.Messages)
	return result
}

// Len returns the number of messages.
func (c *Conversation) Len() int { return len(c.Messages) }

// TokenCount returns the current estimated token count.
func (c *Conversation) TokenCount() int { return EstimateTokens(c.Messages) }

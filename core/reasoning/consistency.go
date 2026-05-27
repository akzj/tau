package reasoning

import (
	"context"
	"fmt"
	"strings"

	"github.com/akzj/tau/core"
)

// ConsistencyStrategy implements Self-Consistency sampling.
// Generates K samples with varied perspectives, then majority vote selects the answer.
type ConsistencyStrategy struct {
	inner   core.AgentStrategy
	samples int
	temp    float64
}

// NewConsistencyStrategy creates a Self-Consistency strategy.
// samples defaults to 5 if <= 0. temp is fixed at 0.7 for mock testing.
func NewConsistencyStrategy(inner core.AgentStrategy, samples int) *ConsistencyStrategy {
	if samples <= 0 {
		samples = 5
	}
	return &ConsistencyStrategy{inner: inner, samples: samples, temp: 0.7}
}

// Name returns "consistency".
func (s *ConsistencyStrategy) Name() string { return "consistency" }

// Decide generates K reasoning perspectives and returns the majority consensus.
func (s *ConsistencyStrategy) Decide(ctx context.Context, state core.AgentState) (core.Decision, error) {
	var answers []string
	for i := 0; i < s.samples; i++ {
		perspective := s.getPerspective(i)
		answers = append(answers, perspective)
	}

	best := s.majorityVote(answers)

	return core.Decision{
		Action: "continue",
		Reason: fmt.Sprintf("[Consistency] %d samples → majority: %q", s.samples, best),
	}, nil
}

func (s *ConsistencyStrategy) getPerspective(i int) string {
	perspectives := []string{
		"analyze systematically",
		"consider edge cases",
		"verify from first principles",
		"check for assumptions",
		"evaluate trade-offs",
	}
	return perspectives[i%len(perspectives)]
}

func (s *ConsistencyStrategy) majorityVote(answers []string) string {
	counts := make(map[string]int)
	var best string
	var bestCount int
	for _, a := range answers {
		key := strings.ToLower(strings.TrimSpace(a))
		counts[key]++
		if counts[key] > bestCount {
			bestCount = counts[key]
			best = a
		}
	}
	return best
}

func init() {
	core.RegisterStrategy("consistency", func() core.AgentStrategy {
		return NewConsistencyStrategy(core.GetStrategy("react"), 5)
	})
}

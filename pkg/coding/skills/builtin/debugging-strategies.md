---
name: "debugging-strategies"
description: "Scientific debugging: bisect, hypothesis-driven, log injection, divide and conquer"
disable-model-invocation: false
trigger: "debug|trace|diagnose|troubleshoot|root cause"
tools: [read, grep, glob, bash]
---

## Strategies
- **Bisect**: Use `git bisect` to find the commit that introduced the bug. Binary search over history.
- **Hypothesis-driven**: Form a hypothesis → design a test → prove/disprove. Don't guess; experiment.
- **Log injection**: Add targeted `fmt.Printf`/`slog.Debug` at decision points. Read the actual values.
- **Divide and conquer**: Comment out half the code. Bug gone? It's in that half. Repeat.
- **Scientific method**: Observe → Hypothesize → Predict → Experiment → Conclude.

## Steps
1. Reproduce the bug reliably. Document the exact steps.
2. Add a failing test that demonstrates the bug.
3. Use bisect or divide-and-conquer to narrow the search space.
4. Read the code at the failure point with fresh eyes. Challenge assumptions.
5. Fix the root cause, not the symptom. Keep the test as regression protection.

package skills

func init() {
	GlobalSkills.Register(&SkillDefinition{
		Name:        "perf_profile",
		Description: "Profile performance, identify hotspots, and optimize critical paths",
		Tools:       []string{"run_bench", "search_code", "read", "find_references", "write"},
		Prompt:      perfProfilePrompt,
		Category:    "performance",
	})
}

const perfProfilePrompt = `Performance profiling process:
1. **Benchmark**: Use run_bench to establish baseline metrics.
2. **Hotspot**: Identify functions with high allocation or time.
3. **Analyze**: Read the hotspot code. Look for:
   - Unnecessary allocations (pre-allocate slices/maps)
   - Repeated work (cache results)
   - Inefficient algorithms (O(n²) → O(n log n))
   - Blocking I/O (add concurrency)
4. **Optimize**: Make ONE targeted change.
5. **Re-benchmark**: Verify improvement. If no improvement, revert.
6. **Document**: Note the optimization and its impact.`
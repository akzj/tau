---
name: "performance-optimization"
description: "Performance optimization: profiling, bottleneck identification, allocation reduction, caching, lazy loading"
disable-model-invocation: false
trigger: "performance|slow|optimize|bottleneck|profile|benchmark|latency"
tools: [read, write, edit, bash, grep, glob]
---

## Process
- **Profile first**: Never optimize without data. `go tool pprof`, `go test -benchmem`. Find the actual bottleneck.
- **Identify bottleneck**: Is it CPU-bound or IO-bound? Where is most time spent? Which allocations dominate?
- **Reduce allocations**: Use `sync.Pool` for reusable objects. Pre-allocate slices with known capacity. Avoid `fmt.Sprintf` in hot paths.
- **Cache**: Cache expensive computations. Use `sync.Map` or `map + sync.RWMutex`. Invalidate cache on write.
- **Lazy loading**: Defer initialization until first use. `sync.Once` for thread-safe lazy init.

## Steps
1. Run benchmark: `go test -bench=. -benchmem ./pkg/...` → identify slowest function.
2. Run CPU profile: `go test -bench=. -cpuprofile=/tmp/cpu.out ./pkg/...` → `go tool pprof /tmp/cpu.out`
3. Run memory profile: `go test -bench=. -memprofile=/tmp/mem.out ./pkg/...` → `go tool pprof /tmp/mem.out`
4. Optimize the top bottleneck. Re-run benchmark to verify improvement.
5. Repeat until performance is acceptable.

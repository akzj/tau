package sandbox

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- Mock backend for unit tests ---

type mockBackend struct {
	name      string
	available bool
	result    *Result
	err       error
	delay     time.Duration
	mu        sync.Mutex
	calls     int
}

func (m *mockBackend) Name() string      { return m.name }
func (m *mockBackend) Available() bool    { return m.available }
func (m *mockBackend) Run(ctx context.Context, cmd string, workDir string, timeout time.Duration) (*Result, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return m.result, m.err
	}
	return m.result, nil
}

// --- Runner tests ---

func TestRunnerBackend(t *testing.T) {
	mb := &mockBackend{name: "mock", available: true}
	r := &Runner{backend: mb, timeout: 10 * time.Second}
	if got := r.Backend(); got != "mock" {
		t.Errorf("Backend() = %q, want %q", got, "mock")
	}
}

func TestRunnerBackendNil(t *testing.T) {
	r := &Runner{}
	if got := r.Backend(); got != "none" {
		t.Errorf("nil backend → %q, want %q", got, "none")
	}
}

func TestRunnerRun(t *testing.T) {
	mb := &mockBackend{
		name:      "mock",
		available: true,
		result:    &Result{Stdout: "hello", ExitCode: 0, Sandboxed: true},
	}
	r := &Runner{backend: mb, timeout: 10 * time.Second}

	result, err := r.Run(context.Background(), "echo hello", "/tmp")
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Stdout != "hello" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "hello")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !result.Sandboxed {
		t.Error("expected Sandboxed=true")
	}
	if result.Duration == "" {
		t.Error("Duration should be populated")
	}
}

func TestRunnerRunExitCode(t *testing.T) {
	mb := &mockBackend{
		name:      "mock",
		available: true,
		result:    &Result{Stdout: "", Stderr: "error", ExitCode: 42, Sandboxed: true},
	}
	r := &Runner{backend: mb, timeout: 10 * time.Second}

	result, err := r.Run(context.Background(), "false", "/tmp")
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", result.ExitCode)
	}
	if result.Stderr != "error" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "error")
	}
}

func TestRunnerRunTimeout(t *testing.T) {
	mb := &mockBackend{
		name:  "slow",
		delay: 5 * time.Second,
		result: &Result{Stdout: "never"},
	}
	r := &Runner{backend: mb, timeout: 50 * time.Millisecond}

	_, err := r.Run(context.Background(), "sleep 10", "/tmp")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestRunnerSetTimeout(t *testing.T) {
	mb := &mockBackend{name: "mock", available: true, result: &Result{Stdout: "ok"}}
	r := &Runner{backend: mb, timeout: 100 * time.Millisecond}
	r.SetTimeout(50 * time.Millisecond)
	// Timeout changed, should still work
	_, err := r.Run(context.Background(), "echo ok", "/tmp")
	if err != nil {
		t.Fatalf("Run after SetTimeout: %v", err)
	}
}

func TestRunnerStop(t *testing.T) {
	r := &Runner{backend: &mockBackend{name: "mock"}}
	if err := r.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

// --- Concurrency test ---

func TestRunnerConcurrent(t *testing.T) {
	mb := &mockBackend{
		name:      "mock",
		available: true,
		result:    &Result{Stdout: "ok", ExitCode: 0, Sandboxed: true},
	}
	r := &Runner{backend: mb, timeout: 10 * time.Second}

	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := r.Run(context.Background(), "echo ok", "/tmp")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Run error: %v", err)
		}
	}
	if mb.calls != 3 {
		t.Errorf("expected 3 calls, got %d", mb.calls)
	}
}

// --- Result tests ---

func TestResultJSONSerialization(t *testing.T) {
	r := &Result{
		Stdout:    "hello world",
		Stderr:    "some error",
		ExitCode:  1,
		Duration:  "1.5s",
		Sandboxed: true,
		Error:     "",
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Stdout != "hello world" {
		t.Errorf("Stdout = %q", decoded.Stdout)
	}
	if decoded.ExitCode != 1 {
		t.Errorf("ExitCode = %d", decoded.ExitCode)
	}
	if !decoded.Sandboxed {
		t.Error("Sandboxed should be true")
	}
	if decoded.Duration != "1.5s" {
		t.Errorf("Duration = %q", decoded.Duration)
	}
}

func TestResultDefaultSandboxed(t *testing.T) {
	r := &Result{}
	if r.Sandboxed {
		t.Error("zero-value Sandboxed should be false")
	}
	if r.ExitCode != 0 {
		t.Errorf("zero-value ExitCode = %d", r.ExitCode)
	}
}

// --- truncateOutput tests ---

func TestTruncateOutputShort(t *testing.T) {
	out := truncateOutput("short")
	if out != "short" {
		t.Errorf("truncateOutput(short) = %q", out)
	}
}

func TestTruncateOutputLong(t *testing.T) {
	long := strings.Repeat("x", 5000)
	out := truncateOutput(long)
	if len(out) <= 4000 {
		t.Errorf("expected >4000 chars due to suffix, got %d", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Error("missing 'truncated' suffix")
	}
}

func TestTruncateOutputExact(t *testing.T) {
	exact := strings.Repeat("x", 4000)
	out := truncateOutput(exact)
	if len(out) != 4000 {
		t.Errorf("exact 4000 chars should not truncate, got %d", len(out))
	}
}

// --- LocalBackend tests ---

func TestLocalBackendAvailable(t *testing.T) {
	lb := NewLocalBackend()
	if !lb.Available() {
		t.Error("LocalBackend should always be available")
	}
}

func TestLocalBackendName(t *testing.T) {
	lb := NewLocalBackend()
	if lb.Name() != "local" {
		t.Errorf("Name() = %q, want %q", lb.Name(), "local")
	}
}

func TestLocalBackendRun(t *testing.T) {
	lb := NewLocalBackend()
	result, err := lb.Run(context.Background(), "echo hello", "", 10*time.Second)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Stdout missing 'hello': %q", result.Stdout)
	}
	if result.Sandboxed {
		t.Error("local execution should have Sandboxed=false")
	}
}

func TestLocalBackendRunExitCode(t *testing.T) {
	lb := NewLocalBackend()
	result, err := lb.Run(context.Background(), "exit 13", "", 10*time.Second)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.ExitCode != 13 {
		t.Errorf("ExitCode = %d, want 13", result.ExitCode)
	}
	if result.Sandboxed {
		t.Error("local execution should have Sandboxed=false")
	}
}

func TestLocalBackendRunStderr(t *testing.T) {
	lb := NewLocalBackend()
	result, err := lb.Run(context.Background(), "echo err >&2", "", 10*time.Second)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(result.Stderr, "err") {
		t.Errorf("Stderr missing 'err': %q", result.Stderr)
	}
}

func TestLocalBackendRunWorkDir(t *testing.T) {
	lb := NewLocalBackend()
	result, err := lb.Run(context.Background(), "pwd", "/tmp", 10*time.Second)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(result.Stdout, "/tmp") {
		t.Errorf("Stdout should contain '/tmp': %q", result.Stdout)
	}
}

func TestLocalBackendRunEmptyCmd(t *testing.T) {
	lb := NewLocalBackend()
	result, err := lb.Run(context.Background(), "", "", 10*time.Second)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("empty cmd ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestLocalBackendRunTimeout(t *testing.T) {
	lb := NewLocalBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result, _ := lb.Run(ctx, "sleep 10", "", 10*time.Second)
	// Timeout/signal should result in non-zero exit code
	if result.ExitCode == 0 {
		t.Errorf("expected non-zero exit code on timeout, got %d", result.ExitCode)
	}
}

// --- DockerBackend tests ---

func TestDockerBackendName(t *testing.T) {
	db := NewDockerBackend()
	if db.Name() != "docker" {
		t.Errorf("Name() = %q, want %q", db.Name(), "docker")
	}
}

func TestDockerBackendAvailable(t *testing.T) {
	db := NewDockerBackend()
	// Docker may or may not be available — just check it doesn't panic
	_ = db.Available()
}

func TestDockerBackendRunNotAvailable(t *testing.T) {
	// Just test that DockerBackend exists and has correct defaults.
	db := NewDockerBackend()
	if db.image != "alpine:latest" {
		t.Errorf("image = %q, want alpine:latest", db.image)
	}
	if db.network != "none" {
		t.Errorf("network = %q, want none", db.network)
	}
	if db.memory != "512m" {
		t.Errorf("memory = %q, want 512m", db.memory)
	}
	if db.cpus != "1" {
		t.Errorf("cpus = %q, want 1", db.cpus)
	}
}

func TestDockerBackendRunWhenAvailable(t *testing.T) {
	db := NewDockerBackend()
	if !db.Available() {
		t.Skip("Docker not available")
	}

	result, err := db.Run(context.Background(), "echo hello", "", 30*time.Second)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("Stdout missing 'hello': %q", result.Stdout)
	}
	if !result.Sandboxed {
		t.Error("Docker execution should have Sandboxed=true")
	}
}

func TestDockerBackendNetworkDisabled(t *testing.T) {
	db := NewDockerBackend()
	if !db.Available() {
		t.Skip("Docker not available")
	}

	result, err := db.Run(context.Background(), "curl -s --connect-timeout 5 https://httpbin.org/get", "", 30*time.Second)
	if err != nil {
		// Container might fail at creation or execution — both are acceptable
		t.Logf("network test (expected fail with --network=none): %v", err)
		return
	}
	// With --network=none, curl should fail
	if result.ExitCode == 0 {
		t.Error("curl should fail with --network=none")
	}
}

func TestDockerBackendExitCode(t *testing.T) {
	db := NewDockerBackend()
	if !db.Available() {
		t.Skip("Docker not available")
	}

	result, err := db.Run(context.Background(), "exit 7", "", 30*time.Second)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.ExitCode != 7 {
		t.Errorf("ExitCode = %d, want 7", result.ExitCode)
	}
}

func TestDockerBackendStderr(t *testing.T) {
	db := NewDockerBackend()
	if !db.Available() {
		t.Skip("Docker not available")
	}

	result, err := db.Run(context.Background(), "echo err >&2", "", 30*time.Second)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(result.Stderr, "err") {
		t.Errorf("Stderr missing 'err': %q", result.Stderr)
	}
}

func TestDockerBackendTimeout(t *testing.T) {
	db := NewDockerBackend()
	if !db.Available() {
		t.Skip("Docker not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, _ := db.Run(ctx, "sleep 30", "", 30*time.Second)
	// Timeout/signal should result in non-zero exit code
	if result.ExitCode == 0 {
		t.Errorf("expected non-zero exit code on timeout, got %d", result.ExitCode)
	}
}

// --- NewRunner integration tests ---

func TestNewRunner(t *testing.T) {
	r := NewRunner()
	if r == nil {
		t.Fatal("NewRunner returned nil")
	}
	backend := r.Backend()
	if backend != "docker" && backend != "local" && backend != "none" {
		t.Errorf("unexpected backend: %q", backend)
	}
	if r.timeout != 300*time.Second {
		t.Errorf("timeout = %v, want 300s", r.timeout)
	}
}

func TestNewRunnerRun(t *testing.T) {
	r := NewRunner()
	result, err := r.Run(context.Background(), "echo hello", "")
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Stdout == "" && result.Stderr == "" {
		t.Error("expected some output")
	}
}

// --- Type constants ---

func TestTypes(t *testing.T) {
	if Docker != "docker" {
		t.Error("Docker constant mismatch")
	}
	if Podman != "podman" {
		t.Error("Podman constant mismatch")
	}
	if None != "none" {
		t.Error("None constant mismatch")
	}
}

// --- Config compatibility (config.go uses Type constants) ---

func TestConfigPreferredBackend(t *testing.T) {
	tests := []struct {
		value string
		want  Type
	}{
		{"docker", Docker},
		{"podman", Podman},
		{"none", None},
		{"", None},
		{"unknown", None},
	}
	for _, tc := range tests {
		cfg := Config{Sandbox: tc.value}
		got := cfg.PreferredBackend()
		if got != tc.want {
			t.Errorf("Config{%q}.PreferredBackend() = %v, want %v", tc.value, got, tc.want)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{Sandbox: ""}
	if cfg.PreferredBackend() != None {
		t.Errorf("empty config → None, got %v", cfg.PreferredBackend())
	}
}
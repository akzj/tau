package persist

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/akzj/tau/core"
)

func BenchmarkSave(b *testing.B) {
	msgs := make([]core.Message, 100)
	for i := 0; i < 100; i++ {
		msgs[i] = core.Message{Role: core.RoleUser, Content: "hello world this is a test message"}
	}
	id := "bench-" + time.Now().Format("150405")
	defer os.Remove(filepath.Join(mustDirBench(b), id+".jsonl"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Save(id, msgs)
	}
}

func BenchmarkLoad(b *testing.B) {
	msgs := make([]core.Message, 100)
	for i := 0; i < 100; i++ {
		msgs[i] = core.Message{Role: core.RoleUser, Content: "hello world this is a test message"}
	}
	id := "bench-load-" + time.Now().Format("150405")
	Save(id, msgs)
	defer os.Remove(filepath.Join(mustDirBench(b), id+".jsonl"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Load(id)
	}
}

func BenchmarkSaveLoad(b *testing.B) {
	msgs := make([]core.Message, 100)
	for i := 0; i < 100; i++ {
		msgs[i] = core.Message{Role: core.RoleUser, Content: "hello world this is a test message"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := "bench-sl-" + time.Now().Format("150405")
		Save(id, msgs)
		Load(id)
		os.Remove(filepath.Join(mustDirBench(b), id+".jsonl"))
	}
}

func mustDirBench(b *testing.B) string {
	b.Helper()
	dir, err := Dir()
	if err != nil {
		b.Fatalf("Dir: %v", err)
	}
	return dir
}

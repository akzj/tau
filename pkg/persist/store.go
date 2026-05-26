package persist

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// Dir returns the sessions directory (~/.tau/sessions), creating it if needed.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	d := filepath.Join(home, ".tau", "sessions")
	if err := os.MkdirAll(d, 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	return d, nil
}

// SessionInfo is metadata about a saved session.
type SessionInfo struct {
	ID        string
	CreatedAt time.Time
	MsgCount  int
	FirstMsg  string // first user message (truncated to 80 chars)
}

// Save writes the transcript messages to a JSONL file.
func Save(id string, msgs []core.Message) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, id+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, m := range msgs {
		if err := enc.Encode(m); err != nil {
			return fmt.Errorf("encode: %w", err)
		}
	}
	return nil
}

// Load reads a JSONL session file and returns the messages.
func Load(id string) ([]core.Message, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, id+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", id, err)
	}
	defer f.Close()

	var msgs []core.Message
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var m core.Message
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			return nil, fmt.Errorf("decode line: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return msgs, nil
}

// List returns all saved session IDs with metadata, sorted newest first.
func List() ([]SessionInfo, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var infos []SessionInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")

		msgs, err := Load(id)
		if err != nil {
			continue
		}

		info := SessionInfo{
			ID:       id,
			MsgCount: len(msgs),
		}
		// Get first user message
		for _, m := range msgs {
			if m.Role == core.RoleUser {
				info.FirstMsg = m.Content
				if len(info.FirstMsg) > 80 {
					info.FirstMsg = info.FirstMsg[:80] + "..."
				}
				break
			}
		}
		// Use file mod time as created time
		fi, err := e.Info()
		if err == nil {
			info.CreatedAt = fi.ModTime()
		}
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].CreatedAt.After(infos[j].CreatedAt)
	})
	return infos, nil
}

// NewID generates a session ID based on timestamp.
func NewID() string {
	return time.Now().Format("20060102-150405")
}
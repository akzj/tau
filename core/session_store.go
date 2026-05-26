package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionStore manages persistent session files in a directory.
type SessionStore struct {
	Dir string
}

// NewSessionStore creates a store at the given directory path.
func NewSessionStore(dir string) *SessionStore {
	return &SessionStore{Dir: dir}
}

// SessionRecord is the on-disk representation of a saved session.
type SessionRecord struct {
	ID         string     `json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Messages   []Message  `json:"messages"`
	Summary    string     `json:"summary,omitempty"`
	CWD        string     `json:"cwd,omitempty"`
	TotalUsage TokenUsage `json:"total_usage,omitempty"`
	CallCount  int        `json:"call_count"`
}

// Save persists a session to a JSON file.
func (s *SessionStore) Save(sess *Session) error {
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return fmt.Errorf("session store mkdir: %w", err)
	}
	path := filepath.Join(s.Dir, string(sess.ID)+".json")
	tmpPath := path + ".tmp"

	record := SessionRecord{
		ID:         string(sess.ID),
		CreatedAt:  sess.CreatedAt,
		UpdatedAt:  time.Now(),
		Messages:   sess.Transcript.Messages(),
		Summary:    sess.Summary,
		CWD:        sess.CWD,
		TotalUsage: sess.TotalUsage,
		CallCount:  sess.CallCount,
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// Load reads a saved session from disk.
func (s *SessionStore) Load(id string) (*SessionRecord, error) {
	path := filepath.Join(s.Dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %s not found", id)
		}
		return nil, fmt.Errorf("read: %w", err)
	}
	var record SessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &record, nil
}

// List returns all saved session records sorted by most recent first.
func (s *SessionStore) List() ([]SessionRecord, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var records []SessionRecord
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		record, err := s.Load(id)
		if err != nil {
			continue
		}
		records = append(records, *record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})
	return records, nil
}

// Delete removes a saved session file.
func (s *SessionStore) Delete(id string) error {
	path := filepath.Join(s.Dir, id+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("session %s not found", id)
	}
	return os.Remove(path)
}

// Prune removes sessions older than the given duration.
func (s *SessionStore) Prune(olderThan time.Duration) (int, error) {
	records, err := s.List()
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-olderThan)
	count := 0
	for _, r := range records {
		if r.UpdatedAt.Before(cutoff) {
			if err := s.Delete(r.ID); err == nil {
				count++
			}
		}
	}
	return count, nil
}

package persist

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/akzj/tau/core"
)

// Fork copies a session's entries to a new session file.
// Returns the new session ID.
func Fork(sourceID string, entries []core.TreeEntry) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	newID := NewID()
	path := filepath.Join(dir, newID+".tree.jsonl")
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("fork create: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return "", fmt.Errorf("fork encode: %w", err)
		}
	}
	return newID, nil
}

// LoadTree loads tree entries from a .tree.jsonl file.
func LoadTree(id string) ([]core.TreeEntry, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, id+".tree.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open tree %s: %w", id, err)
	}
	defer f.Close()

	var entries []core.TreeEntry
	dec := json.NewDecoder(f)
	for dec.More() {
		var e core.TreeEntry
		if err := dec.Decode(&e); err != nil {
			return entries, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

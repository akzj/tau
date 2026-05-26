package core

import "time"

// EntryType classifies a session tree entry.
type EntryType string

const (
	// EntryMessage is a transcript message entry.
	EntryMessage EntryType = "message"
	// EntryCompaction is a compaction summary entry.
	EntryCompaction EntryType = "compaction"
	// EntryLeaf is a leaf node in the session tree.
	EntryLeaf EntryType = "leaf"
	// EntryBranchSummary is a branch summary entry.
	EntryBranchSummary EntryType = "branch_summary"
	// EntrySessionInfo is a session metadata entry.
	EntrySessionInfo EntryType = "session_info"
)

// TreeEntry is a node in the session tree. See also HookSet.BeforeSessionTree.
type TreeEntry struct {
	ID        string
	ParentID  string // empty for root
	Type      EntryType
	Timestamp time.Time
	Data      any // the actual payload (Message, CompactionRequest, etc.)
}

// NewTreeEntry creates a tree entry with a generated ID.
func NewTreeEntry(parentID string, typ EntryType, data any) TreeEntry {
	return TreeEntry{
		ID:        generateID(),
		ParentID:  parentID,
		Type:      typ,
		Timestamp: time.Now(),
		Data:      data,
	}
}

package core

import (
	"database/sql"
	"encoding/json"
	"sort"
	"sync"
)

// VectorEntry stores an embedding with associated metadata.
type VectorEntry struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Chunk     string    `json:"chunk"`
	Embedding Embedding `json:"embedding"`
}

// VectorStore provides vector storage and similarity search.
type VectorStore interface {
	Insert(entry VectorEntry) error
	Search(query Embedding, topK int) ([]VectorEntry, error)
	Delete(id string) error
	Count() int
}

// SQLiteVectorStore stores vector embeddings as JSON BLOBs in SQLite.
// It reuses the existing SQLiteStore's database connection.
type SQLiteVectorStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteVectorStore creates a vector store backed by an existing SQLiteStore.
// The vectors table is created if it doesn't exist.
func NewSQLiteVectorStore(store *SQLiteStore) (*SQLiteVectorStore, error) {
	vs := &SQLiteVectorStore{db: store.db}
	if _, err := store.db.Exec(`CREATE TABLE IF NOT EXISTS vectors (
		id TEXT PRIMARY KEY,
		text TEXT NOT NULL,
		chunk TEXT NOT NULL DEFAULT '',
		embedding BLOB NOT NULL
	)`); err != nil {
		return nil, err
	}
	return vs, nil
}

// Insert adds or replaces a vector entry.
func (vs *SQLiteVectorStore) Insert(entry VectorEntry) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	embJSON, err := json.Marshal(entry.Embedding)
	if err != nil {
		return err
	}
	_, err = vs.db.Exec(
		"INSERT OR REPLACE INTO vectors(id,text,chunk,embedding) VALUES(?,?,?,?)",
		entry.ID, entry.Text, entry.Chunk, embJSON,
	)
	return err
}

// Search finds the top-K most similar vectors to the query embedding.
func (vs *SQLiteVectorStore) Search(query Embedding, topK int) ([]VectorEntry, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	if topK <= 0 {
		topK = 5
	}

	rows, err := vs.db.Query("SELECT id,text,chunk,embedding FROM vectors")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		entry VectorEntry
		score float64
	}
	var results []scored

	for rows.Next() {
		var e VectorEntry
		var embJSON []byte
		if err := rows.Scan(&e.ID, &e.Text, &e.Chunk, &embJSON); err != nil {
			continue
		}
		if err := json.Unmarshal(embJSON, &e.Embedding); err != nil {
			continue
		}
		score := CosineSimilarity(query, e.Embedding)
		results = append(results, scored{e, score})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	if topK > len(results) {
		topK = len(results)
	}
	out := make([]VectorEntry, topK)
	for i := 0; i < topK; i++ {
		out[i] = results[i].entry
	}
	return out, nil
}

// Delete removes a vector entry by ID.
func (vs *SQLiteVectorStore) Delete(id string) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	_, err := vs.db.Exec("DELETE FROM vectors WHERE id=?", id)
	return err
}

// Count returns the total number of vectors in the store.
func (vs *SQLiteVectorStore) Count() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	var n int
	vs.db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&n)
	return n
}

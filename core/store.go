package core

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// StorageBackend defines the interface for persistent storage.
// Implementations include SQLiteStore (persistent) and nil (in-memory only).
type StorageBackend interface {
	Write(key string, value []byte) error
	Read(key string) ([]byte, error)
	List(prefix string) ([]string, error)
	Delete(key string) error
	Close() error

	// Episode persistence
	SaveEpisode(ep Episode) error
	LoadEpisodes() ([]Episode, error)
	DeleteEpisode(id string) error
}

// SQLiteStore implements StorageBackend using a pure-Go SQLite driver.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates a SQLite database at the given path.
// Uses WAL journal mode for better concurrency and busy_timeout to reduce lock contention.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS kv (
			key   TEXT PRIMARY KEY,
			value BLOB NOT NULL
		);
		CREATE TABLE IF NOT EXISTS episodes (
			id        TEXT PRIMARY KEY,
			trigger   TEXT NOT NULL,
			action    TEXT NOT NULL,
			outcome   TEXT NOT NULL,
			lesson    TEXT NOT NULL,
			tags      TEXT NOT NULL DEFAULT '',
			timestamp TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_episodes_tags ON episodes(tags);
		CREATE INDEX IF NOT EXISTS idx_episodes_timestamp ON episodes(timestamp);
	`)
	return err
}

// DB returns the underlying *sql.DB for reuse by other components (e.g. cache, vector store).
func (s *SQLiteStore) DB() *sql.DB { return s.db }

// --- KV CRUD ---

func (s *SQLiteStore) Write(key string, value []byte) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO kv(key,value) VALUES(?,?)", key, value)
	return err
}

func (s *SQLiteStore) Read(key string) ([]byte, error) {
	var val []byte
	err := s.db.QueryRow("SELECT value FROM kv WHERE key=?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return val, err
}

func (s *SQLiteStore) List(prefix string) ([]string, error) {
	rows, err := s.db.Query("SELECT key FROM kv WHERE key LIKE ?", prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *SQLiteStore) Delete(key string) error {
	_, err := s.db.Exec("DELETE FROM kv WHERE key=?", key)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- Episode persistence ---

func (s *SQLiteStore) SaveEpisode(ep Episode) error {
	tags := strings.Join(ep.Tags, ",")
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO episodes(id,trigger,action,outcome,lesson,tags,timestamp) VALUES(?,?,?,?,?,?,?)",
		ep.ID, ep.Trigger, ep.Action, ep.Outcome, ep.Lesson, tags, ep.Timestamp.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) LoadEpisodes() ([]Episode, error) {
	rows, err := s.db.Query("SELECT id,trigger,action,outcome,lesson,tags,timestamp FROM episodes ORDER BY timestamp ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var eps []Episode
	for rows.Next() {
		var ep Episode
		var tagsStr, ts string
		if err := rows.Scan(&ep.ID, &ep.Trigger, &ep.Action, &ep.Outcome, &ep.Lesson, &tagsStr, &ts); err != nil {
			return nil, err
		}
		if tagsStr != "" {
			ep.Tags = strings.Split(tagsStr, ",")
		}
		ep.Timestamp, _ = time.Parse(time.RFC3339, ts)
		eps = append(eps, ep)
	}
	return eps, rows.Err()
}

func (s *SQLiteStore) DeleteEpisode(id string) error {
	_, err := s.db.Exec("DELETE FROM episodes WHERE id=?", id)
	return err
}

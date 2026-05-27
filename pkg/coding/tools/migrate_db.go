package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/akzj/tau/core"
	_ "modernc.org/sqlite"
)

// MigrateDBTool runs simple DB migrations from SQL files.
//
// Parameters:
//
//	migration_dir (string, required) — directory with .up.sql / .down.sql migration files
//	action        (string, required) — up | down | status
//
// Migration file naming: 001_name.up.sql, 001_name.down.sql
//
// Tracks applied migrations in a __migrations table in SQLite.
func MigrateDBTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"migration_dir": {"type": "string", "description": "Directory containing migration SQL files"},
			"action": {"type": "string", "description": "Action: up, down, status"}
		},
		"required": ["migration_dir", "action"]
	}`)

	return core.Tool{
		Name:        "migrate_db",
		Description: "Simple DB migration runner. Reads .up.sql/.down.sql files, tracks applied migrations in SQLite table. Supports up/down/status actions.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				MigrationDir string `json:"migration_dir"`
				Action       string `json:"action"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.MigrationDir == "" {
				return core.ToolResult{}, fmt.Errorf("migration_dir required")
			}
			if args.Action == "" {
				return core.ToolResult{}, fmt.Errorf("action required (up/down/status)")
			}

			resolved, err := ResolvePath(args.MigrationDir)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("resolve path: %w", err)
			}

			// Open SQLite DB in the migration dir
			dbPath := filepath.Join(resolved, ".migration.db")
			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("open db: %w", err)
			}
			defer db.Close()

			// Ensure migrations table exists
			if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS __migrations (
				id INTEGER PRIMARY KEY,
				name TEXT UNIQUE NOT NULL,
				applied_at TEXT NOT NULL DEFAULT (datetime('now'))
			)`); err != nil {
				return core.ToolResult{}, fmt.Errorf("create migrations table: %w", err)
			}

			// List migration pairs
			migrations, err := listMigrations(resolved)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("list migrations: %w", err)
			}

			// Get applied migrations
			applied, err := getAppliedMigrations(db)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read applied: %w", err)
			}

			switch args.Action {
			case "status":
				return migrationStatus(migrations, applied, args.MigrationDir)
			case "up":
				return migrationUp(db, migrations, applied, args.MigrationDir)
			case "down":
				return migrationDown(db, migrations, applied, args.MigrationDir)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use up/down/status)", args.Action)
			}
		},
	}
}

type migration struct {
	Prefix string
	Name   string
	UpSQL  string
	DownSQL string
}

var migrationRe = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

func listMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	pairs := make(map[string]*migration)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matches := migrationRe.FindStringSubmatch(e.Name())
		if matches == nil {
			continue
		}
		key := matches[1] + "_" + matches[2]
		if _, ok := pairs[key]; !ok {
			pairs[key] = &migration{Prefix: matches[1], Name: matches[2]}
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if matches[3] == "up" {
			pairs[key].UpSQL = string(data)
		} else {
			pairs[key].DownSQL = string(data)
		}
	}

	var result []migration
	for _, m := range pairs {
		if m.UpSQL != "" {
			result = append(result, *m)
		}
	}

	// Sort by prefix
	sort.Slice(result, func(i, j int) bool {
		ni, _ := strconv.Atoi(result[i].Prefix)
		nj, _ := strconv.Atoi(result[j].Prefix)
		return ni < nj
	})

	return result, nil
}

func getAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("SELECT name FROM __migrations ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

func migrationStatus(migrations []migration, applied map[string]bool, dir string) (core.ToolResult, error) {
	if len(migrations) == 0 {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No migrations found in " + dir}},
			Details: map[string]any{"migration_dir": dir, "total": 0, "applied": 0, "pending": 0},
		}, nil
	}

	appliedCount := 0
	var lines []string
	lines = append(lines, fmt.Sprintf("## Migration Status: %s\n", dir))

	for _, m := range migrations {
		status := "pending"
		if applied[m.Prefix+"_"+m.Name] {
			status = "applied"
			appliedCount++
		}
		lines = append(lines, fmt.Sprintf("  [%s] %s_%s", status, m.Prefix, m.Name))
	}

	output := strings.Join(lines, "\n")

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"migration_dir": dir,
			"total":         len(migrations),
			"applied":       appliedCount,
			"pending":       len(migrations) - appliedCount,
			"success":       true,
		},
	}, nil
}

func migrationUp(db *sql.DB, migrations []migration, applied map[string]bool, dir string) (core.ToolResult, error) {
	var appliedCount int
	var appliedNames []string

	for _, m := range migrations {
		key := m.Prefix + "_" + m.Name
		if applied[key] {
			continue
		}

		// Run in transaction
		tx, err := db.Begin()
		if err != nil {
			return core.ToolResult{}, fmt.Errorf("begin tx: %w", err)
		}

		if _, err := tx.Exec(m.UpSQL); err != nil {
			tx.Rollback()
			return core.ToolResult{}, fmt.Errorf("migration %s up failed: %w", key, err)
		}

		if _, err := tx.Exec("INSERT INTO __migrations (name) VALUES (?)", key); err != nil {
			tx.Rollback()
			return core.ToolResult{}, fmt.Errorf("record migration %s: %w", key, err)
		}

		if err := tx.Commit(); err != nil {
			return core.ToolResult{}, fmt.Errorf("commit %s: %w", key, err)
		}

		applied[key] = true
		appliedCount++
		appliedNames = append(appliedNames, key)
	}

	if appliedCount == 0 {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "All migrations already applied."}},
			Details: map[string]any{"migration_dir": dir, "applied_count": 0, "success": true},
		}, nil
	}

	output := fmt.Sprintf("## Migration Up: %s\n\nApplied %d migration(s):\n", dir, appliedCount)
	for _, n := range appliedNames {
		output += fmt.Sprintf("  ✓ %s\n", n)
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"migration_dir": dir,
			"applied_count": appliedCount,
			"applied":       appliedNames,
			"success":       true,
		},
	}, nil
}

func migrationDown(db *sql.DB, migrations []migration, applied map[string]bool, dir string) (core.ToolResult, error) {
	// Rollback the last applied migration
	// Find the last applied migration
	var lastKey string
	var lastMigration *migration
	for i := len(migrations) - 1; i >= 0; i-- {
		key := migrations[i].Prefix + "_" + migrations[i].Name
		if applied[key] {
			lastKey = key
			lastMigration = &migrations[i]
			break
		}
	}

	if lastMigration == nil {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No migrations to roll back."}},
			Details: map[string]any{"migration_dir": dir, "success": true},
		}, nil
	}

	if lastMigration.DownSQL == "" {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: fmt.Sprintf("No down migration for %s", lastKey)}},
			Details: map[string]any{"migration_dir": dir, "success": false},
		}, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("begin tx: %w", err)
	}

	if _, err := tx.Exec(lastMigration.DownSQL); err != nil {
		tx.Rollback()
		return core.ToolResult{}, fmt.Errorf("migration %s down failed: %w", lastKey, err)
	}

	if _, err := tx.Exec("DELETE FROM __migrations WHERE name = ?", lastKey); err != nil {
		tx.Rollback()
		return core.ToolResult{}, fmt.Errorf("delete record %s: %w", lastKey, err)
	}

	if err := tx.Commit(); err != nil {
		return core.ToolResult{}, fmt.Errorf("commit: %w", err)
	}

	output := fmt.Sprintf("## Migration Down: %s\n\nRolled back: %s", dir, lastKey)

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"migration_dir": dir,
			"rolled_back":   lastKey,
			"success":       true,
		},
	}, nil
}

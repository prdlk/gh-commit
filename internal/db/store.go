// Package db persists per-repository scopes in a pure-Go SQLite database
// (modernc.org/sqlite, no CGO). The schema mirrors the old DuckDB layout,
// with foreign keys + ON DELETE CASCADE replacing the manual cascade.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    gitignore_hash TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS scopes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_id, name)
);
CREATE TABLE IF NOT EXISTS scope_paths (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scope_id INTEGER NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    UNIQUE(scope_id, path)
);
CREATE TABLE IF NOT EXISTS github_labels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scope_id INTEGER NOT NULL UNIQUE REFERENCES scopes(id) ON DELETE CASCADE,
    label_name TEXT NOT NULL,
    color TEXT,
    synced_at TIMESTAMP
);
`

// Dir returns the data directory: ${XDG_DATA_HOME:-~/.local/share}/gh-commit.
func Dir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "gh-commit")
}

// Path returns the SQLite database path.
func Path() string { return filepath.Join(Dir(), "gh-commit.sqlite") }

// RepoInfo is one row of the `list` table.
type RepoInfo struct {
	Name       string
	Path       string
	ScopeCount int
}

// Store wraps the SQLite connection.
type Store struct {
	db *sql.DB
	// HadLegacyDuckDB is true when this Open created the database for the
	// first time while an old DuckDB file was present next to it.
	HadLegacyDuckDB bool
}

// Open creates the data directory, opens the database, and applies the schema.
func Open() (*Store, error) {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}
	path := Path()

	legacy := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(dir, "gh-commit.db")); err == nil {
			legacy = true
		}
	}

	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	if _, err := conn.Exec(schema); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}
	return &Store{db: conn, HadLegacyDuckDB: legacy}, nil
}

// Close closes the underlying connection.
func (s *Store) Close() error { return s.db.Close() }

// HasScopes reports whether repoPath has any stored scopes.
func (s *Store) HasScopes(repoPath string) (bool, error) {
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM scopes s
		JOIN repositories r ON s.repo_id = r.id
		WHERE r.path = ?`, repoPath).Scan(&n)
	return n > 0, err
}

// Scopes returns repoPath's scopes as name -> ordered path prefixes.
func (s *Store) Scopes(repoPath string) (map[string][]string, error) {
	rows, err := s.db.Query(`
		SELECT s.name, sp.path
		FROM scopes s
		JOIN repositories r ON s.repo_id = r.id
		JOIN scope_paths sp ON sp.scope_id = s.id
		WHERE r.path = ?
		ORDER BY s.name, sp.path`, repoPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	scopes := map[string][]string{}
	for rows.Next() {
		var name, path string
		if err := rows.Scan(&name, &path); err != nil {
			return nil, err
		}
		scopes[name] = append(scopes[name], path)
	}
	return scopes, rows.Err()
}

// SaveScopes replaces repoPath's scopes and snapshots gitignoreHash.
func (s *Store) SaveScopes(repoPath, repoName string, scopes map[string][]string, gitignoreHash string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	var repoID int64
	err = tx.QueryRow(`SELECT id FROM repositories WHERE path = ?`, repoPath).Scan(&repoID)
	switch err {
	case nil:
		// ON DELETE CASCADE removes scope_paths and github_labels.
		if _, err := tx.Exec(`DELETE FROM scopes WHERE repo_id = ?`, repoID); err != nil {
			return err
		}
	case sql.ErrNoRows:
		res, err := tx.Exec(`INSERT INTO repositories (path, name) VALUES (?, ?)`, repoPath, repoName)
		if err != nil {
			return err
		}
		if repoID, err = res.LastInsertId(); err != nil {
			return err
		}
	default:
		return err
	}

	names := make([]string, 0, len(scopes))
	for name := range scopes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		res, err := tx.Exec(`INSERT INTO scopes (repo_id, name) VALUES (?, ?)`, repoID, name)
		if err != nil {
			return err
		}
		scopeID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for _, path := range scopes[name] {
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO scope_paths (scope_id, path) VALUES (?, ?)`,
				scopeID, path,
			); err != nil {
				return err
			}
		}
	}

	if _, err := tx.Exec(
		`UPDATE repositories SET updated_at = CURRENT_TIMESTAMP, gitignore_hash = ? WHERE id = ?`,
		gitignoreHash, repoID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// StoredGitignoreHash returns the snapshotted hash for repoPath. present is
// false when the repo is unknown or the hash was never recorded.
func (s *Store) StoredGitignoreHash(repoPath string) (hash string, present bool, err error) {
	var v sql.NullString
	err = s.db.QueryRow(`SELECT gitignore_hash FROM repositories WHERE path = ?`, repoPath).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v.String, v.Valid, nil
}

// ListRepos returns all repositories, most recently updated first.
func (s *Store) ListRepos() ([]RepoInfo, error) {
	rows, err := s.db.Query(`
		SELECT r.name, r.path, COUNT(DISTINCT s.id) AS scope_count
		FROM repositories r
		LEFT JOIN scopes s ON s.repo_id = r.id
		GROUP BY r.id, r.name, r.path, r.updated_at
		ORDER BY r.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var repos []RepoInfo
	for rows.Next() {
		var r RepoInfo
		if err := rows.Scan(&r.Name, &r.Path, &r.ScopeCount); err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

// DeleteRepo removes repoPath and, via cascade, all of its children.
func (s *Store) DeleteRepo(repoPath string) error {
	_, err := s.db.Exec(`DELETE FROM repositories WHERE path = ?`, repoPath)
	return err
}

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/prdlk/gh-commit/internal/ai"
	"github.com/prdlk/gh-commit/internal/gitx"
	"github.com/prdlk/gh-commit/internal/ui"
)

// archiveMigrated renames a consumed legacy file to *.migrated.<timestamp>.
func archiveMigrated(path string) (string, error) {
	backup := path + ".migrated." + time.Now().Format("20060102_150405")
	return backup, os.Rename(path, backup)
}

// migrateTOML imports scopes from .github/Repo.toml.
func migrateTOML(repoPath, repoName, tomlPath string) bool {
	ui.Warnf("↻ Migrating .github/Repo.toml → database")
	var doc struct {
		Scopes map[string]any `toml:"scopes"`
	}
	if _, err := toml.DecodeFile(tomlPath, &doc); err != nil {
		ui.Errorf("Migration failed: %s", err)
		return false
	}
	scopes := ai.CoerceScopeMap(doc.Scopes)
	if scopes == nil {
		scopes = map[string][]string{}
	}
	if err := store.SaveScopes(repoPath, repoName, scopes, gitx.GitignoreHash(repoPath)); err != nil {
		ui.Errorf("Migration failed: %s", err)
		return false
	}
	backup, err := archiveMigrated(tomlPath)
	if err != nil {
		ui.Errorf("Migration failed: %s", err)
		return false
	}
	ui.Dimf("  Archived: %s", backup)
	return true
}

// migrateJSON imports scopes from .github/scopes.json
// (an array of {"scope": ..., "path": ...} entries).
func migrateJSON(repoPath, repoName, jsonPath string) bool {
	ui.Warnf("↻ Migrating .github/scopes.json → database")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		ui.Errorf("Migration failed: %s", err)
		return false
	}
	var entries []struct {
		Scope string `json:"scope"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		ui.Errorf("Migration failed: %s", err)
		return false
	}
	scopes := map[string][]string{}
	for _, e := range entries {
		scopes[e.Scope] = append(scopes[e.Scope], e.Path)
	}
	if err := store.SaveScopes(repoPath, repoName, scopes, gitx.GitignoreHash(repoPath)); err != nil {
		ui.Errorf("Migration failed: %s", err)
		return false
	}
	backup, err := archiveMigrated(jsonPath)
	if err != nil {
		ui.Errorf("Migration failed: %s", err)
		return false
	}
	ui.Dimf("  Archived: %s", backup)
	return true
}

// migrateLegacy imports whichever legacy config file exists, if any.
func migrateLegacy(repoPath string) bool {
	repoName := filepath.Base(repoPath)
	if tomlPath := filepath.Join(repoPath, ".github", "Repo.toml"); fileExists(tomlPath) {
		return migrateTOML(repoPath, repoName, tomlPath)
	}
	if jsonPath := filepath.Join(repoPath, ".github", "scopes.json"); fileExists(jsonPath) {
		return migrateJSON(repoPath, repoName, jsonPath)
	}
	return false
}

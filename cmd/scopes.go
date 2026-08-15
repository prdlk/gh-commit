package cmd

import (
	"sort"
	"strings"

	"github.com/prdlk/gh-commit/internal/ai"
	"github.com/prdlk/gh-commit/internal/gitx"
	"github.com/prdlk/gh-commit/internal/ui"
)

// sortedScopeNames returns scope names in deterministic (sorted) order,
// matching the DB query ordering the Python tool iterated in.
func sortedScopeNames(scopes map[string][]string) []string {
	names := make([]string, 0, len(scopes))
	for name := range scopes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// displayScopes prints "• name: path, path" bullets, sorted by name.
func displayScopes(scopes map[string][]string) {
	for _, name := range sortedScopeNames(scopes) {
		ui.Printf("  %s %s: %s",
			ui.Cyan("•"), ui.Bold(name), strings.Join(scopes[name], ", "))
	}
}

// generateScopes builds the file tree, prompts the model, and parses the
// response. Failures are reported here; nil means "keep going without".
func generateScopes(existing map[string][]string) map[string][]string {
	filetree := gitx.Filetree()
	if filetree == "" {
		ui.Errorf("No tracked or untracked files to analyze")
		return nil
	}
	var out string
	var err error
	ui.Spin("Analyzing repository with Ollama...", func() {
		out, err = client.ScopesRaw(filetree, existing)
	})
	if err != nil {
		ui.Errorf("Ollama failed: %s", err)
		return nil
	}
	scopes := ai.ParseScopesResponse(out)
	if scopes == nil {
		ui.Errorf("Could not parse scopes from Ollama's response")
		if cfg.debug {
			ui.Dimf("%s", out)
		}
	}
	return scopes
}

// maybeAutoRefreshScopes regenerates scopes when .gitignore changed since the
// last save. First sighting snapshots a baseline without regenerating.
func maybeAutoRefreshScopes(repoPath, repoName string) error {
	if cfg.noAutoRefresh {
		return nil
	}
	current := gitx.GitignoreHash(repoPath)
	stored, present, err := store.StoredGitignoreHash(repoPath)
	if err != nil {
		return err
	}
	if !present {
		scopes, err := store.Scopes(repoPath)
		if err != nil {
			return err
		}
		return store.SaveScopes(repoPath, repoName, scopes, current)
	}
	if current == stored {
		return nil
	}

	ui.Warnf("↻ .gitignore changed — regenerating scopes with Ollama...")
	existing, err := store.Scopes(repoPath)
	if err != nil {
		return err
	}
	scopes := generateScopes(existing)
	if scopes == nil {
		ui.Dimf("  Keeping existing scopes (regeneration failed)")
		ui.Println()
		return nil
	}
	if err := store.SaveScopes(repoPath, repoName, scopes, current); err != nil {
		return err
	}
	ui.Successf("✓ Scopes updated:")
	displayScopes(scopes)
	ui.Println()
	return nil
}

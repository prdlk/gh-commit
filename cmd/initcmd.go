package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/prdlk/gh-commit/internal/gitx"
	"github.com/prdlk/gh-commit/internal/ui"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate scopes for this repository from its file tree",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		repoPath, err := requireGit()
		if err != nil {
			return err
		}
		if err := ensureAI(); err != nil {
			return err
		}
		return initFlow(repoPath)
	},
}

// initFlow generates and saves scopes for repoPath. The Ollama client must
// already be ready.
func initFlow(repoPath string) error {
	repoName := filepath.Base(repoPath)

	// Check for legacy files.
	tomlPath := filepath.Join(repoPath, ".github", "Repo.toml")
	jsonPath := filepath.Join(repoPath, ".github", "scopes.json")
	if fileExists(tomlPath) || fileExists(jsonPath) {
		ui.Warnf("Found legacy config file(s)")
		if ui.Confirm("Migrate to the gh-commit database?") {
			if migrateLegacy(repoPath) {
				ui.Successf("✓ Migration complete")
				return nil
			}
		}
	}

	hasScopes, err := store.HasScopes(repoPath)
	if err != nil {
		return err
	}
	if hasScopes {
		ui.Warnf("⚠ Repository already configured")
		if !ui.Confirm("Overwrite existing scopes?") {
			ui.Dimf("Cancelled")
			return nil
		}
	}

	ui.MagentaBoldf("Generating scopes for %s...", repoName)
	ui.Println()
	scopes := generateScopes(nil)
	if scopes == nil {
		return errAborted
	}

	if err := store.SaveScopes(repoPath, repoName, scopes, gitx.GitignoreHash(repoPath)); err != nil {
		return err
	}
	ui.Successf("✓ Saved scopes to database")
	ui.Println()
	ui.Magentaf("Generated scopes:")
	displayScopes(scopes)
	ui.Println()
	ui.Dimf("Run 'gh commit' to use these scopes")
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func init() {
	rootCmd.AddCommand(initCmd)
}

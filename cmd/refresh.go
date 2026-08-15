package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/prdlk/gh-commit/internal/gitx"
	"github.com/prdlk/gh-commit/internal/ui"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Regenerate scopes from the current repository structure",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		repoPath, err := requireGit()
		if err != nil {
			return err
		}
		repoName := filepath.Base(repoPath)

		hasScopes, err := store.HasScopes(repoPath)
		if err != nil {
			return err
		}
		if !hasScopes {
			ui.Errorf("Repository not configured — run 'gh commit init' first")
			return errAborted
		}
		if err := ensureAI(); err != nil {
			return err
		}

		existing, err := store.Scopes(repoPath)
		if err != nil {
			return err
		}
		ui.MagentaBoldf("Refreshing scopes for %s...", repoName)
		ui.Println()
		ui.Infof("Current scopes:")
		displayScopes(existing)
		ui.Println()

		if !ui.Confirm("Refresh scopes based on current structure?") {
			ui.Dimf("Cancelled")
			return nil
		}

		scopes := generateScopes(existing)
		if scopes == nil {
			return errAborted
		}

		ui.Println()
		ui.Successf("✓ Generated updated scopes")
		ui.Println()
		ui.Magentaf("Updated scopes:")
		displayScopes(scopes)
		ui.Println()

		if ui.Confirm("Apply these changes?") {
			if err := store.SaveScopes(repoPath, repoName, scopes, gitx.GitignoreHash(repoPath)); err != nil {
				return err
			}
			ui.Successf("✓ Updated scopes")
		} else {
			ui.Dimf("Changes not applied")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(refreshCmd)
}

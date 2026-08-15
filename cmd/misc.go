package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/prdlk/gh-commit/internal/db"
	"github.com/prdlk/gh-commit/internal/ui"
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the current repository from the database",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		repoPath, err := requireGit()
		if err != nil {
			return err
		}
		hasScopes, err := store.HasScopes(repoPath)
		if err != nil {
			return err
		}
		if !hasScopes {
			ui.Dimf("Repository not in database: %s", filepath.Base(repoPath))
			return nil
		}
		if ui.Confirm(fmt.Sprintf("Remove %s from database?", filepath.Base(repoPath))) {
			if err := store.DeleteRepo(repoPath); err != nil {
				return err
			}
			ui.Successf("✓ Removed %s", filepath.Base(repoPath))
		} else {
			ui.Dimf("Cancelled")
		}
		return nil
	},
}

var dbPathCmd = &cobra.Command{
	Use:   "db-path",
	Short: "Print the database file path",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println(db.Path())
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("gh-commit %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(removeCmd, dbPathCmd, versionCmd)
}

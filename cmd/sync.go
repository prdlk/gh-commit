package cmd

import (
	"crypto/md5" //nolint:gosec // non-cryptographic: stable label color derivation
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/prdlk/gh-commit/internal/ui"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync scopes to GitHub labels",
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
			ui.Errorf("Repository not configured — run 'gh commit init' first")
			return errAborted
		}

		if _, err := exec.LookPath("gh"); err != nil {
			ui.Errorf("Error: 'gh' command not found")
			return errAborted
		}
		if exec.Command("gh", "repo", "view").Run() != nil {
			ui.Errorf("Error: Not a GitHub repository or not authenticated")
			return errAborted
		}

		ui.MagentaBoldf("Syncing scopes → GitHub labels...")
		ui.Println()
		scopes, err := store.Scopes(repoPath)
		if err != nil {
			return err
		}

		created, updated, failed := 0, 0, 0
		for _, name := range sortedScopeNames(scopes) {
			desc := "Changes to: " + strings.Join(scopes[name], ", ")
			sum := md5.Sum([]byte(name)) //nolint:gosec // see import note
			color := hex.EncodeToString(sum[:])[:6]

			if exec.Command("gh", "label", "create", name, "--description", desc, "--color", color).Run() == nil {
				ui.Printf("  %s Created: %s", ui.Green("✓"), name)
				created++
			} else if exec.Command("gh", "label", "edit", name, "--description", desc, "--color", color).Run() == nil {
				ui.Printf("  %s Updated: %s", ui.Yellow("↻"), name)
				updated++
			} else {
				ui.Printf("  %s Failed: %s", ui.Red("✗"), name)
				failed++
			}
		}

		ui.Println()
		ui.SuccessBoldf("Sync complete! %s", fmt.Sprintf("Created: %d | Updated: %d | Failed: %d", created, updated, failed))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

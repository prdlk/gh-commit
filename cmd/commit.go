package cmd

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/prdlk/gh-commit/internal/gitx"
	"github.com/prdlk/gh-commit/internal/ui"
)

// runCommit is the root command: the full scoped-commit flow.
func runCommit(_ *cobra.Command, _ []string) error {
	repoPath, err := requireGit()
	if err != nil {
		return err
	}
	repoName := filepath.Base(repoPath)

	migrateLegacy(repoPath)

	hasScopes, err := store.HasScopes(repoPath)
	if err != nil {
		return err
	}
	if !hasScopes {
		ui.Println()
		ui.WarnBoldf("⚠ No scopes configured for this repository")
		ui.Println()
		ui.Infof("gh-commit organizes commits by project areas (scopes).")
		ui.Println()
		if !ui.Confirm("Generate scopes now using Ollama?") {
			ui.Println()
			ui.Dimf("Run 'gh commit init' to configure scopes")
			return errAborted
		}
		if err := ensureAI(); err != nil {
			return err
		}
		if err := initFlow(repoPath); err != nil {
			return err
		}
	}

	if err := ensureAI(); err != nil {
		return err
	}

	// Keep scopes aligned with the repo whenever .gitignore changes.
	if err := maybeAutoRefreshScopes(repoPath, repoName); err != nil {
		return err
	}

	ui.Magentaf("Finding scopes with changes...")
	changedFiles := gitx.ChangedFiles()
	scopes, err := store.Scopes(repoPath)
	if err != nil {
		return err
	}

	var scopesWithChanges []string
	for _, name := range sortedScopeNames(scopes) {
		if len(gitx.FilesInScope(changedFiles, scopes[name])) > 0 {
			scopesWithChanges = append(scopesWithChanges, name)
		}
	}

	if len(scopesWithChanges) == 0 {
		ui.Dimf("No scoped changes found")
	} else {
		ui.Infof("Scopes with changes: %s", strings.Join(scopesWithChanges, " "))
		ui.Println()
	}

	for _, scope := range scopesWithChanges {
		ui.MagentaBoldf("Processing scope: %s", scope)
		ui.Infof("  Paths: %s", strings.Join(scopes[scope], ", "))
		// Re-read changed files so earlier commits are excluded.
		scopeFiles := gitx.FilesInScope(gitx.ChangedFiles(), scopes[scope])
		if len(scopeFiles) == 0 {
			ui.Dimf("  No files found in scope paths")
			ui.Println()
			continue
		}
		if _, err := commitGroup(scopeFiles, scope); err != nil {
			return err
		}
	}

	gitx.ResetStaging()

	// Remaining files outside any scope.
	if gitx.Git("status", "--porcelain") != "" {
		ui.Warnf("Processing remaining files outside any scope...")
		ui.Println()

		tracked := gitx.Lines(gitx.Git("diff", "--name-only"))
		if len(tracked) > 0 {
			ui.Magentaf("Tracked unstaged files:")
			for _, f := range tracked {
				ui.Dimf("  %s", f)
			}
			if ui.Confirm("Commit tracked unstaged files?") {
				if _, err := commitGroup(tracked, ""); err != nil {
					return err
				}
			}
		}

		untracked := gitx.Lines(gitx.Git("ls-files", "--others", "--exclude-standard"))
		if len(untracked) > 0 {
			ui.Magentaf("Untracked files:")
			for _, f := range untracked {
				ui.Dimf("  %s", f)
			}
			if ui.Confirm("Commit untracked files?") {
				if _, err := commitGroup(untracked, ""); err != nil {
					return err
				}
			}
		}
	}

	// Final safety reset before the push check.
	gitx.ResetStaging()

	// Push.
	unpushed := gitx.UnpushedCommits()
	if len(unpushed) > 0 {
		ui.Println()
		ui.MagentaBoldf("Unpushed Commits")
		ui.Infof("%d commit(s) ready to push:", len(unpushed))
		ui.Println()
		for _, line := range unpushed {
			hash, rest, _ := strings.Cut(line, " ")
			ui.Printf("  %s %s", ui.Bold(hash), rest)
		}
		ui.Println()
		if cfg.push || ui.Confirm("Push commits to origin?") {
			if err := pushToOrigin(); err != nil {
				return err
			}
		} else {
			ui.Dimf("Skipped push")
		}
	} else {
		ui.Println()
		ui.Dimf("No unpushed commits")
	}

	ui.Println()
	ui.SuccessBoldf("✓ Done!")
	return nil
}

// commitGroup stages files, generates a message, and commits them as one group.
func commitGroup(files []string, scope string) (bool, error) {
	var clean []string
	for _, f := range files {
		if f != "" {
			clean = append(clean, f)
		}
	}
	if len(clean) == 0 {
		return false, nil
	}

	gitx.ResetStaging()
	if err := gitx.StageFiles(clean); err != nil {
		return false, err
	}
	diff := gitx.Git("diff", "--cached")
	if diff == "" {
		gitx.ResetStaging()
		return false, nil
	}

	var message string
	ui.Spin("Generating commit message...", func() {
		var err error
		message, err = client.CommitMessage(diff, scope)
		if err != nil {
			ui.Errorf("Ollama failed to generate a commit message: %s", err)
			message = ""
		}
	})
	if message == "" {
		gitx.ResetStaging()
		return false, nil
	}

	ui.Panel(message)
	label := scope
	if label == "" {
		label = "these changes"
	}
	if ui.Confirm("Commit " + label + "?") {
		if err := gitx.Commit(message); err != nil {
			return false, err
		}
		ui.Successf("✓ Committed %s", label)
		ui.Println()
		return true, nil
	}
	gitx.ResetStaging()
	ui.Dimf("  Skipped %s", label)
	ui.Println()
	return false, nil
}

// pushToOrigin pushes the current branch behind a spinner.
func pushToOrigin() error {
	branch := gitx.CurrentBranch()
	var err error
	ui.Spin("Pushing to origin/"+branch+"...", func() {
		err = gitx.Push(branch)
	})
	if err != nil {
		return err
	}
	ui.Successf("✓ Pushed to origin/%s", branch)
	return nil
}

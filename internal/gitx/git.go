// Package gitx wraps the git CLI. Read helpers swallow errors and return ""
// (mirroring the Python tool); mutating helpers pass git output through and
// return errors.
package gitx

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Git runs git with args and returns trimmed stdout, or "" on any failure.
func Git(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// passthrough runs git with args, streaming output to the terminal.
func passthrough(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// IsRepo reports whether the working directory is inside a git work tree.
func IsRepo() bool {
	return exec.Command("git", "rev-parse", "--is-inside-work-tree").Run() == nil
}

// RepoRoot returns the repository top-level path, or "" outside a repo.
func RepoRoot() string { return Git("rev-parse", "--show-toplevel") }

// Lines splits git output into non-empty lines.
func Lines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// ChangedFiles returns staged + unstaged + untracked files, deduplicated and sorted.
func ChangedFiles() []string {
	seen := map[string]struct{}{}
	for _, out := range []string{
		Git("diff", "--cached", "--name-only"),
		Git("diff", "--name-only"),
		Git("ls-files", "--others", "--exclude-standard"),
	} {
		for _, f := range Lines(out) {
			seen[f] = struct{}{}
		}
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// FilesInScope returns the files whose path starts with any of the scope prefixes.
func FilesInScope(files, prefixes []string) []string {
	var matched []string
	for _, f := range files {
		for _, p := range prefixes {
			if strings.HasPrefix(f, p) {
				matched = append(matched, f)
				break
			}
		}
	}
	return matched
}

// StageFiles stages files with git add.
func StageFiles(files []string) error {
	if len(files) == 0 {
		return nil
	}
	return passthrough(append([]string{"add"}, files...)...)
}

// ResetStaging unstages everything (git reset HEAD -- .), ignoring errors.
func ResetStaging() {
	_ = exec.Command("git", "reset", "HEAD", "--", ".").Run()
}

// Commit commits the staging area with message.
func Commit(message string) error { return passthrough("commit", "-m", message) }

// UnpushedCommits returns oneline entries for commits not on any remote.
func UnpushedCommits() []string {
	return Lines(Git("log", "--branches", "--not", "--remotes", "--oneline"))
}

// CurrentBranch returns the checked-out branch name.
func CurrentBranch() string { return Git("branch", "--show-current") }

// Push pushes branch to origin.
func Push(branch string) error { return passthrough("push", "origin", branch) }

// Filetree returns a sorted, deduplicated repo-relative file listing
// (tracked + non-ignored untracked), one path per line.
func Filetree() string {
	seen := map[string]struct{}{}
	for _, out := range []string{
		Git("ls-files"),
		Git("ls-files", "--others", "--exclude-standard"),
	} {
		for _, f := range Lines(out) {
			seen[f] = struct{}{}
		}
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return strings.Join(files, "\n")
}

// GitignoreHash returns the SHA-256 hex digest of the repo's .gitignore,
// or "" when there is none.
func GitignoreHash(repoRoot string) string {
	data, err := os.ReadFile(filepath.Join(repoRoot, ".gitignore"))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

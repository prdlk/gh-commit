// Package diff filters git diffs before they reach the model: lock/generated
// files lose their hunk bodies, JSON changes are capped, and every other file
// is truncated. Behavior is a verbatim port of the Python filter_diff.
package diff

import (
	"fmt"
	"regexp"
	"strings"
)

var lockPattern = regexp.MustCompile(
	`(package-lock\.json|yarn\.lock|pnpm-lock\.yaml|bun\.lockb|` +
		`go\.sum|go\.mod|Cargo\.lock|poetry\.lock|composer\.lock|Gemfile\.lock|` +
		`.*\.min\.(js|css)|.*\.bundle\.js|.*\.map|` +
		`dist/.*|build/.*|\.next/.*|node_modules/.*|vendor/.*|__pycache__/.*|\.pyc$|target/.*)`,
)

var (
	fileNameRe = regexp.MustCompile(`b/([^ ]+)`)
	headerRe   = regexp.MustCompile(`^(index|---|\+\+\+|@@)`)
)

const (
	// MaxDiffLines caps the per-file line count for regular files.
	MaxDiffLines = 200
	// MaxJSONLines caps the +/- line count for JSON files.
	MaxJSONLines = 50
)

// Filter rewrites a raw git diff for prompt consumption.
func Filter(diff string) string {
	var lines []string
	inFiltered, inJSON := false, false
	lineCount, jsonCount := 0, 0

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			lineCount, jsonCount = 0, 0
			filename := ""
			if m := fileNameRe.FindStringSubmatch(line); m != nil {
				filename = m[1]
			}

			switch {
			case lockPattern.MatchString(filename):
				inFiltered, inJSON = true, false
				lines = append(lines, line)
				continue
			case strings.HasSuffix(filename, ".json"):
				inFiltered, inJSON = false, true
				lines = append(lines, line)
				continue
			default:
				inFiltered, inJSON = false, false
			}
		}

		// Lock/generated files: keep headers, replace each hunk body with a
		// marker. (The Python original stopped filtering after the first @@,
		// leaking hunk bodies; the spec requires bodies replaced, so filtering
		// holds until the next "diff --git".)
		if inFiltered {
			if headerRe.MatchString(line) {
				lines = append(lines, line)
				if strings.HasPrefix(line, "@@") {
					lines = append(lines, "[Generated/lock file - content filtered]")
				}
			}
			continue
		}

		if inJSON {
			if headerRe.MatchString(line) {
				lines = append(lines, line)
				continue
			}
			if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
				jsonCount++
				if jsonCount <= MaxJSONLines {
					lines = append(lines, line)
				} else if jsonCount == MaxJSONLines+1 {
					lines = append(lines, fmt.Sprintf("[... JSON truncated after %d lines ...]", MaxJSONLines))
				}
			} else {
				lines = append(lines, line)
			}
			continue
		}

		if lineCount < MaxDiffLines {
			lines = append(lines, line)
			lineCount++
		} else if lineCount == MaxDiffLines {
			lines = append(lines, fmt.Sprintf("[... truncated after %d lines ...]", MaxDiffLines))
			lineCount++
		}
	}

	return strings.Join(lines, "\n")
}

package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	thinkRe      = regexp.MustCompile(`(?s)<think>.*?</think>`)
	fenceOpenRe  = regexp.MustCompile("^```[a-zA-Z]*\n")
	fenceCloseRe = regexp.MustCompile("\n```$")
	scopedMsgRe  = regexp.MustCompile(`^\w+\([^)]+\): .+`)
	plainMsgRe   = regexp.MustCompile(`^\w+: .+`)
)

// stripThink removes <think>...</think> reasoning blocks.
func stripThink(text string) string {
	return strings.TrimSpace(thinkRe.ReplaceAllString(text, ""))
}

// ExtractJSONObject returns the first balanced top-level {...} object in text,
// or "" when none is found.
func ExtractJSONObject(text string) string {
	depth := 0
	start := -1
	for i := range len(text) {
		switch text[i] {
		case '{':
			if start == -1 {
				start = i
			}
			depth++
		case '}':
			if start != -1 {
				depth--
				if depth == 0 {
					return text[start : i+1]
				}
			}
		}
	}
	return ""
}

// CoerceScopeMap normalizes a decoded JSON/TOML mapping into scope -> paths.
// String values become single-element slices; empty paths are dropped.
// Returns nil when nothing usable remains.
func CoerceScopeMap(data map[string]any) map[string][]string {
	scopes := map[string][]string{}
	for name, paths := range data {
		switch v := paths.(type) {
		case string:
			scopes[name] = []string{v}
		case []any:
			var out []string
			for _, p := range v {
				s := fmt.Sprint(p)
				if strings.TrimSpace(s) != "" {
					out = append(out, s)
				}
			}
			scopes[name] = out
		}
	}
	if len(scopes) == 0 {
		return nil
	}
	return scopes
}

// ParseScopesResponse extracts and normalizes the scope mapping from a model
// response. Returns nil when no usable JSON object is present.
func ParseScopesResponse(text string) map[string][]string {
	blob := ExtractJSONObject(text)
	if blob == "" {
		return nil
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(blob), &data); err != nil {
		return nil
	}
	return CoerceScopeMap(data)
}

// CleanCommitMessage normalizes a raw model response into a commit message:
// trims, strips <think> blocks and code fences, prefers the first non-empty
// line when it matches conventional-commit shape, and otherwise falls back to
// the whole trimmed string (never blocks a commit on a regex).
func CleanCommitMessage(raw string) string {
	msg := stripThink(strings.TrimSpace(raw))
	msg = fenceOpenRe.ReplaceAllString(msg, "")
	msg = fenceCloseRe.ReplaceAllString(msg, "")
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if scopedMsgRe.MatchString(line) || plainMsgRe.MatchString(line) {
			return line
		}
		break
	}
	return msg
}

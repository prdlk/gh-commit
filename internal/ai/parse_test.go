package ai

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"bare object", `{"a": 1}`, `{"a": 1}`},
		{"leading prose", `Here you go: {"a": {"b": 2}} done`, `{"a": {"b": 2}}`},
		{"nested braces balanced", `{"a": {"b": {"c": 3}}}`, `{"a": {"b": {"c": 3}}}`},
		{"first object wins", `{"a": 1} {"b": 2}`, `{"a": 1}`},
		{"no object", "no json here", ""},
		{"unbalanced", `{"a": 1`, ""},
		{"stray close before open", `} {"a": 1}`, `{"a": 1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractJSONObject(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseScopesResponse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string][]string
	}{
		{
			"array values",
			`{"core": ["src/", "lib/"], "docs": ["README.md"]}`,
			map[string][]string{"core": {"src/", "lib/"}, "docs": {"README.md"}},
		},
		{
			"string value coerced to slice",
			`{"cli": "cmd/"}`,
			map[string][]string{"cli": {"cmd/"}},
		},
		{
			"empty paths dropped",
			`{"core": ["src/", "", "  "]}`,
			map[string][]string{"core": {"src/"}},
		},
		{
			"surrounding prose ignored",
			"Sure! Here are the scopes:\n{\"api\": [\"api/\"]}\nHope that helps.",
			map[string][]string{"api": {"api/"}},
		},
		{"not an object", `["a", "b"]`, nil},
		{"invalid json", `{"a": }`, nil},
		{"empty object", `{}`, nil},
		{"no json at all", "nothing", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseScopesResponse(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCleanCommitMessage(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"clean scoped message", "feat(core): add thing", "feat(core): add thing"},
		{"clean unscoped message", "chore: tidy", "chore: tidy"},
		{"whitespace trimmed", "  fix(db): close conn  \n", "fix(db): close conn"},
		{
			"code fence stripped",
			"```\nfeat(ui): add button\n```",
			"feat(ui): add button",
		},
		{
			"language fence stripped",
			"```text\nfix(api): handle 404\n```",
			"fix(api): handle 404",
		},
		{
			"think block stripped",
			"<think>\nLet me reason about this diff...\n</think>\nfeat(ai): wire ollama",
			"feat(ai): wire ollama",
		},
		{
			"rambling reduced to first valid line",
			"feat(core): add parser\n\nThis commit introduces a parser.",
			"feat(core): add parser",
		},
		{
			"invalid message falls back to whole string",
			"I could not produce a message",
			"I could not produce a message",
		},
		{
			"multiline invalid first line falls back to whole string",
			"Here is the commit:\nnot conventional",
			"Here is the commit:\nnot conventional",
		},
		{"empty input", "", ""},
		{"only think block", "<think>hmm</think>", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanCommitMessage(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildCommitPrompt(t *testing.T) {
	p := buildCommitPrompt("DIFF_BODY", "core")
	for _, want := range []string{"Use this scope: core", "DIFF_BODY", "Do not think. Answer immediately."} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if p2 := buildCommitPrompt("DIFF_BODY", ""); strings.Contains(p2, "Use this scope") {
		t.Error("scope hint should be absent without a scope")
	}
}

func TestBuildScopePrompt(t *testing.T) {
	p := buildScopePrompt("a.go\nb.go", nil)
	for _, want := range []string{"a.go\nb.go", `{"scope": ["path", ...], ...}`, "Do not think. Answer immediately."} {
		if !strings.Contains(p, want) {
			t.Errorf("new prompt missing %q", want)
		}
	}
	if strings.Contains(p, "Existing scopes") {
		t.Error("new prompt must not mention existing scopes")
	}

	p = buildScopePrompt("a.go", map[string][]string{"core": {"src/"}})
	for _, want := range []string{"Existing scopes (JSON):", `"core"`, `"src/"`} {
		if !strings.Contains(p, want) {
			t.Errorf("update prompt missing %q", want)
		}
	}
}

package diff

import (
	"fmt"
	"strings"
	"testing"
)

func diffHeader(file string) string {
	return fmt.Sprintf("diff --git a/%s b/%s\nindex 111..222 100644\n--- a/%s\n+++ b/%s\n@@ -1,2 +1,2 @@",
		file, file, file, file)
}

func TestFilter(t *testing.T) {
	bigBody := make([]string, 0, 300)
	for i := range 300 {
		bigBody = append(bigBody, fmt.Sprintf("+line %d", i))
	}

	jsonBody := make([]string, 0, 80)
	for i := range 80 {
		jsonBody = append(jsonBody, fmt.Sprintf("+  \"key%d\": %d,", i, i))
	}

	tests := []struct {
		name        string
		input       string
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "small file passes through unchanged",
			input:       diffHeader("main.go") + "\n-old\n+new",
			wantContain: []string{"-old", "+new"},
			wantAbsent:  []string{"truncated", "filtered"},
		},
		{
			name:  "lock file body replaced with marker",
			input: diffHeader("package-lock.json") + "\n-\"old\": 1\n+\"new\": 2",
			wantContain: []string{
				"diff --git a/package-lock.json b/package-lock.json",
				"@@ -1,2 +1,2 @@",
				"[Generated/lock file - content filtered]",
			},
			wantAbsent: []string{`"old": 1`, `"new": 2`},
		},
		{
			name:        "go.sum matched by lock pattern",
			input:       diffHeader("go.sum") + "\n+github.com/x v1.0.0 h1:abc",
			wantContain: []string{"[Generated/lock file - content filtered]"},
			wantAbsent:  []string{"h1:abc"},
		},
		{
			name:        "path prefix matched anywhere in name",
			input:       diffHeader("node_modules/foo.js") + "\n+secret",
			wantContain: []string{"[Generated/lock file - content filtered]"},
			wantAbsent:  []string{"+secret"},
		},
		{
			name:  "json file capped at 50 +/- lines",
			input: diffHeader("config.json") + "\n" + strings.Join(jsonBody, "\n"),
			wantContain: []string{
				`+  "key49": 49,`,
				"[... JSON truncated after 50 lines ...]",
			},
			wantAbsent: []string{`"key50"`, `"key79"`},
		},
		{
			name:        "json context lines kept beyond the cap",
			input:       diffHeader("config.json") + "\n" + strings.Join(jsonBody, "\n") + "\n context line",
			wantContain: []string{" context line"},
		},
		{
			name:  "regular file truncated at 200 lines",
			input: diffHeader("big.go") + "\n" + strings.Join(bigBody, "\n"),
			wantContain: []string{
				"[... truncated after 200 lines ...]",
			},
			wantAbsent: []string{"+line 299"},
		},
		{
			name: "counters reset per file",
			input: diffHeader("big.go") + "\n" + strings.Join(bigBody, "\n") + "\n" +
				diffHeader("small.go") + "\n+after",
			wantContain: []string{"+after"},
		},
		{
			name: "lock file followed by normal file",
			input: diffHeader("yarn.lock") + "\n+lockline\n" +
				diffHeader("app.ts") + "\n+visible",
			wantContain: []string{"[Generated/lock file - content filtered]", "+visible"},
			wantAbsent:  []string{"+lockline"},
		},
		{
			name:        "empty diff stays empty",
			input:       "",
			wantContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter(tt.input)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\noutput:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("output should not contain %q\noutput:\n%s", absent, got)
				}
			}
		})
	}
}

func TestFilterTruncationMarkerOnlyOnce(t *testing.T) {
	var body []string
	for i := range 400 {
		body = append(body, fmt.Sprintf("+l%d", i))
	}
	got := Filter(diffHeader("big.go") + "\n" + strings.Join(body, "\n"))
	if n := strings.Count(got, "[... truncated after 200 lines ...]"); n != 1 {
		t.Errorf("want exactly 1 truncation marker, got %d", n)
	}
}

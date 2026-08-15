package gitx

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFilesInScope(t *testing.T) {
	files := []string{
		"cmd/root.go",
		"cmd/list.go",
		"internal/db/store.go",
		"README.md",
		"cmdlets/x.go",
	}
	tests := []struct {
		name     string
		prefixes []string
		want     []string
	}{
		{"directory prefix", []string{"cmd/"}, []string{"cmd/root.go", "cmd/list.go"}},
		{"bare prefix matches sibling dirs too", []string{"cmd"}, []string{"cmd/root.go", "cmd/list.go", "cmdlets/x.go"}},
		{"exact file", []string{"README.md"}, []string{"README.md"}},
		{"multiple prefixes no duplicates", []string{"cmd/", "cmd/root"}, []string{"cmd/root.go", "cmd/list.go"}},
		{"no match", []string{"docs/"}, nil},
		{"empty prefixes", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilesInScope(files, tt.prefixes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitignoreHash(t *testing.T) {
	dir := t.TempDir()

	if got := GitignoreHash(dir); got != "" {
		t.Errorf("missing .gitignore should hash to empty string, got %q", got)
	}

	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// sha256 of "node_modules/\n"
	first := GitignoreHash(dir)
	if len(first) != 64 {
		t.Errorf("want 64-char hex digest, got %q", first)
	}
	if again := GitignoreHash(dir); again != first {
		t.Error("hash must be deterministic")
	}

	if err := os.WriteFile(path, []byte("dist/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed := GitignoreHash(dir); changed == first {
		t.Error("hash must change when content changes")
	}
}

func TestLines(t *testing.T) {
	if got := Lines("a\n\nb\n"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("got %v", got)
	}
	if got := Lines(""); got != nil {
		t.Errorf("empty output should yield nil, got %v", got)
	}
}

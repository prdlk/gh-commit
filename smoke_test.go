//go:build integration

// Integration smoke test: builds the binary, then runs init + commit in a
// temp git repo against a live Ollama server. Skipped automatically when the
// server (or the model) is unavailable.
//
//	go test -tags integration -run TestSmoke -v .
package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ollamaHost() string {
	if h := os.Getenv("GH_COMMIT_OLLAMA_HOST"); h != "" {
		return h
	}
	return "http://localhost:11434"
}

func run(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func TestSmoke(t *testing.T) {
	client := &http.Client{Timeout: 2 * time.Second}
	if _, err := client.Get(ollamaHost() + "/api/tags"); err != nil {
		t.Skipf("Ollama not reachable at %s: %v", ollamaHost(), err)
	}

	bin := filepath.Join(t.TempDir(), "gh-commit")
	run(t, ".", nil, "go", "build", "-o", bin, ".")

	repo := t.TempDir()
	env := []string{
		"GH_COMMIT_AUTO=1",
		"XDG_DATA_HOME=" + t.TempDir(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=smoke", "GIT_AUTHOR_EMAIL=smoke@test",
		"GIT_COMMITTER_NAME=smoke", "GIT_COMMITTER_EMAIL=smoke@test",
	}
	run(t, repo, env, "git", "init", "-q")
	origin := t.TempDir()
	run(t, origin, env, "git", "init", "-q", "--bare")
	run(t, repo, env, "git", "remote", "add", "origin", origin)

	writeFile := func(rel, content string) {
		t.Helper()
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("src/app.go", "package app\n\nfunc Run() {}\n")
	writeFile("docs/README.md", "# Smoke\n")

	out := run(t, repo, env, bin, "init")
	if !strings.Contains(out, "Saved scopes to database") {
		t.Fatalf("init did not save scopes:\n%s", out)
	}

	writeFile("src/app.go", "package app\n\nfunc Run() { println(\"changed\") }\n")

	out = run(t, repo, env, bin)
	if !strings.Contains(out, "✓ Done!") {
		t.Fatalf("commit flow did not complete:\n%s", out)
	}

	log := run(t, repo, env, "git", "log", "--format=%s")
	if strings.TrimSpace(log) == "" {
		t.Fatalf("no commits created; gh-commit output:\n%s", out)
	}
	t.Logf("commits:\n%s", log)
}

package db

import (
	"reflect"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	s, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSaveAndReadScopes(t *testing.T) {
	s := openTestStore(t)
	scopes := map[string][]string{
		"core": {"internal/", "main.go"},
		"docs": {"README.md"},
	}

	if has, _ := s.HasScopes("/repo"); has {
		t.Fatal("fresh store should have no scopes")
	}
	if err := s.SaveScopes("/repo", "repo", scopes, "hash1"); err != nil {
		t.Fatal(err)
	}
	if has, _ := s.HasScopes("/repo"); !has {
		t.Fatal("scopes should exist after save")
	}
	got, err := s.Scopes("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, scopes) {
		t.Errorf("got %#v, want %#v", got, scopes)
	}

	hash, present, err := s.StoredGitignoreHash("/repo")
	if err != nil || !present || hash != "hash1" {
		t.Errorf("want (hash1, true), got (%q, %v, %v)", hash, present, err)
	}
}

func TestSaveScopesReplacesExisting(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveScopes("/repo", "repo", map[string][]string{"old": {"a/"}}, "h1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveScopes("/repo", "repo", map[string][]string{"new": {"b/"}}, "h2"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Scopes("/repo")
	want := map[string][]string{"new": {"b/"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("old scopes must be cascade-deleted; got %#v", got)
	}
	if hash, _, _ := s.StoredGitignoreHash("/repo"); hash != "h2" {
		t.Errorf("hash not updated: %q", hash)
	}
}

func TestDeleteRepoCascades(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveScopes("/repo", "repo", map[string][]string{"core": {"a/"}}, "h"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRepo("/repo"); err != nil {
		t.Fatal(err)
	}
	if has, _ := s.HasScopes("/repo"); has {
		t.Error("scopes must be gone after repo delete")
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM scope_paths`).Scan(&n); err != nil || n != 0 {
		t.Errorf("scope_paths rows must cascade away, got %d (%v)", n, err)
	}
}

func TestStoredGitignoreHashUnknownRepo(t *testing.T) {
	s := openTestStore(t)
	if _, present, err := s.StoredGitignoreHash("/nowhere"); err != nil || present {
		t.Errorf("unknown repo: want (false, nil), got (%v, %v)", present, err)
	}
}

func TestListRepos(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveScopes("/a", "a", map[string][]string{"x": {"x/"}, "y": {"y/"}}, ""); err != nil {
		t.Fatal(err)
	}
	repos, err := s.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "a" || repos[0].ScopeCount != 2 {
		t.Errorf("got %#v", repos)
	}
}

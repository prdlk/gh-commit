# gh-commit

AI-powered scoped git commits as a GitHub CLI extension, driven by the
[Crush](https://github.com/charmbracelet/crush) CLI. Scopes map Conventional
Commit scope names to path prefixes. They are generated from the repository
file tree, stored in a local DuckDB database, and auto-regenerate whenever
`.gitignore` changes. Changed files are grouped by scope and committed one
scope at a time with generated `type(scope): message` subjects.

## Install

```sh
gh extension install prdlk/gh-commit
```

### Requirements

- [uv](https://docs.astral.sh/uv) — Python package runner (handles dependencies automatically)
- [Crush](https://github.com/charmbracelet/crush) — configured with an authenticated model provider

By default, gh-commit uses `openrouter/qwen/qwen3.6-27b`. Configure OpenRouter in Crush or override the model with `GH_COMMIT_CRUSH_MODEL`.

## Usage

```sh
cd your-repo
gh commit init   # generate scopes for this repo
gh commit        # commit changes grouped by scope
```

## Commands

| Command | Behavior |
|---|---|
| `gh commit` | Full scoped-commit flow: group changed files by scope, generate a message per group, confirm, commit, offer to push |
| `gh commit init` | Generate scopes from the file tree via Crush; migrates legacy configs; confirms overwrite |
| `gh commit refresh` | Show current scopes, regenerate with existing scopes as context, confirm apply |
| `gh commit sync` | Create/update GitHub labels from scopes (color = first 6 hex chars of MD5 of the scope name) |
| `gh commit list` | Table of configured repositories: name, path, scope count |
| `gh commit remove` | Delete the current repo from the database after confirmation |
| `gh commit db-path` | Print the database file path |
| `gh commit version` | Print `gh-commit <version>` |
| `gh commit help` | Help, including the DB path and environment variable docs |

Flags: `--auto` (skip confirmations), `--push` (auto-push).

## Environment

| Variable | Description |
|----------|-------------|
| `GH_COMMIT_AUTO=1` | Skip all confirmation prompts |
| `GH_COMMIT_PUSH=1` | Auto-push after commits |
| `GH_COMMIT_NO_AUTO_REFRESH=1` | Don't auto-regenerate scopes when `.gitignore` changes |
| `GH_COMMIT_CRUSH_CMD` | Override the Crush command (default `crush`) |
| `GH_COMMIT_CRUSH_MODEL` | Override the Crush model (default `openrouter/qwen/qwen3.6-27b`) |
| `GH_COMMIT_CRUSH_TIMEOUT` | Per-prompt timeout in seconds (default `120`) |
| `GH_COMMIT_DEBUG=1` | Show scope-response parse diagnostics |

## Storage

Scopes live in `${XDG_DATA_HOME:-~/.local/share}/gh-commit/gh-commit.db`
(DuckDB). Legacy `.github/Repo.toml` and `.github/scopes.json` files are
migrated automatically on first run (the source file is archived as
`*.migrated.<timestamp>`).

## Manual acceptance

```sh
mkdir /tmp/accept && cd /tmp/accept && git init
mkdir -p src docs
echo 'package app' > src/app.go
echo '# Docs' > docs/README.md
gh commit init          # scopes generated and saved
echo '// change' >> src/app.go
echo 'More docs' >> docs/README.md
gh commit               # one commit per scope
git log --format='%s'   # verify type(scope): message subjects
```

## Development

The extension is a bash launcher (`gh-commit`) that runs `smartcommit.py`
with `uv run --script`; uv resolves the inline dependencies (duckdb, rich,
questionary) automatically.

```sh
python -m py_compile smartcommit.py   # syntax check
bash -n gh-commit                     # launcher check
./gh-commit version                   # smoke test
```

# gh-commit

AI-powered scoped git commits as a GitHub CLI extension, driven by a local
Ollama model. Scopes map Conventional Commit scope names to path prefixes.
They are generated from the repository file tree, stored in a local SQLite
database, and auto-regenerate whenever `.gitignore` changes. Changed files are
grouped by scope and committed one scope at a time with generated
`type(scope): message` subjects.

## Install

```sh
# Prerequisites: git, a running Ollama server, and the model
ollama pull qwen3.5:2b

gh extension install prdlk/gh-commit
```

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
| `gh commit init` | Generate scopes from the file tree via Ollama; migrates legacy configs; confirms overwrite |
| `gh commit refresh` | Show current scopes, regenerate with existing scopes as context, confirm apply |
| `gh commit sync` | Create/update GitHub labels from scopes (color = first 6 hex chars of MD5 of the scope name) |
| `gh commit list` | Table of configured repositories: name, path, scope count |
| `gh commit remove` | Delete the current repo from the database after confirmation |
| `gh commit db-path` | Print the database file path |
| `gh commit version` | Print `gh-commit <version>` |
| `gh commit help` | Help, including the DB path and environment variable docs |

Flags: `--auto` (skip confirmations), `--push` (auto-push), `--model`, `--host`.

## Environment

| Variable | Default | Purpose |
|---|---|---|
| `GH_COMMIT_OLLAMA_HOST` | `http://localhost:11434` | Ollama server URL |
| `GH_COMMIT_MODEL` | `qwen3.5:2b` | Model tag |
| `GH_COMMIT_TIMEOUT` | `120` | Per-request timeout, seconds |
| `GH_COMMIT_AUTO` | `0` | `1` = skip all confirmations |
| `GH_COMMIT_PUSH` | `0` | `1` = auto-push after commits |
| `GH_COMMIT_NO_AUTO_REFRESH` | `0` | `1` = never auto-regenerate scopes |
| `GH_COMMIT_DEBUG` | `0` | `1` = print raw model output on parse failure |

## Storage

Scopes live in `${XDG_DATA_HOME:-~/.local/share}/gh-commit/gh-commit.sqlite`.
Legacy `.github/Repo.toml` and `.github/scopes.json` files are migrated
automatically on first run (the source file is archived as
`*.migrated.<timestamp>`). Databases from the old DuckDB-based Python version
are not migrated; rerun `gh commit init`.

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

```sh
go test ./...                              # unit tests
go test -tags integration -run TestSmoke . # needs a running Ollama server
go build .
```

Releases are built by `cli/gh-extension-precompile` on `v*` tags for
linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, and windows/amd64
(`CGO_ENABLED=0`; the SQLite driver is pure Go).

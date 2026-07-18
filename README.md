# gh-commit

AI-powered scoped git commits. Groups changes by project area, generates commit messages with `crush run`, and pushes — all in one command.

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
# Initialize scopes for your repo (uses Crush to analyze structure)
gh commit init

# Commit changes grouped by scope
gh commit

# Auto-confirm + auto-push
gh commit --auto --push

# Manually refresh scopes after structural changes
gh commit refresh

# Sync scopes as GitHub labels
gh commit sync
```

## How it works

1. **`gh commit init`** — `crush run` analyzes your repo structure and generates scope definitions (e.g., `core → src/`, `docs → docs/, README.md`, `ci → .github/workflows/`)
2. **`gh commit`** — Groups dirty files by scope, sends each staged diff to `crush run`, and commits each group separately
3. **Auto-refresh** — Whenever your `.gitignore` changes, scopes are automatically regenerated before committing (a content hash of `.gitignore` is tracked per repo)
4. Remaining unscoped files are handled in a final pass
5. Unpushed commits are offered for push

Scopes are stored in a local DuckDB database (`~/.local/share/gh-commit/gh-commit.db`) — no config files in your repo.

### Why Crush?

The former Mods roles now live directly in `smartcommit.py`, so gh-commit no longer depends on Mods configuration. Each generation invokes Crush's supported non-interactive mode with a self-contained prompt.

## Commands

| Command | Description |
|---------|-------------|
| `gh commit` | Commit changes grouped by scope |
| `gh commit init` | Generate scopes for current repo |
| `gh commit refresh` | Update scopes from current structure |
| `gh commit sync` | Sync scopes → GitHub labels |
| `gh commit list` | List all configured repositories |
| `gh commit remove` | Remove current repo from database |
| `gh commit db-path` | Print database file path |
| `gh commit version` | Print version |
| `gh commit help` | Show help |

## Flags

| Flag | Description |
|------|-------------|
| `--auto` | Skip all confirmation prompts |
| `--push` | Auto-push after committing |

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

## Migration

Existing `.github/Repo.toml` or `.github/scopes.json` files are automatically detected and migrated to DuckDB on first run.

## License

MIT

# gh-commit

AI-powered scoped git commits. Groups changes by project area, generates commit messages with Claude, and pushes — all in one command.

## Install

```sh
gh extension install prdlk/gh-commit
```

### Requirements

- [uv](https://docs.astral.sh/uv) — Python package runner (handles dependencies automatically)
- [mods](https://github.com/charmbracelet/mods) — LLM CLI for commit message generation
- [claude](https://docs.anthropic.com/en/docs/claude-code) — Claude Code CLI for scope generation

## Usage

```sh
# Initialize scopes for your repo (uses Claude to analyze structure)
gh commit init

# Commit changes grouped by scope
gh commit

# Auto-confirm + auto-push
gh commit --auto --push

# Refresh scopes after structural changes
gh commit refresh

# Sync scopes as GitHub labels
gh commit sync
```

## How it works

1. **`gh commit init`** — Claude analyzes your repo structure and generates scope definitions (e.g., `core → src/`, `docs → docs/, README.md`, `ci → .github/workflows/`)
2. **`gh commit`** — Groups dirty files by scope, generates a commit message per scope via `mods`, and commits each group separately
3. Remaining unscoped files are handled in a final pass
4. Unpushed commits are offered for push

Scopes are stored in a local DuckDB database (`~/.local/share/gh-commit/gh-commit.db`) — no config files in your repo.

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
| `GH_COMMIT_MODS_CMD` | Override the mods command string |

## Migration

Existing `.github/Repo.toml` or `.github/scopes.json` files are automatically detected and migrated to DuckDB on first run.

## License

MIT

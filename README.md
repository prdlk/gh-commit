# gh-commit

AI-powered scoped git commits. Groups changes by project area, generates commit messages with Claude, and pushes — all in one command. A single Claude agent, reached over the **Agent Client Protocol (ACP)**, handles both scope generation and commit-message writing.

## Install

```sh
gh extension install prdlk/gh-commit
```

### Requirements

- [uv](https://docs.astral.sh/uv) — Python package runner (handles dependencies automatically)
- [Node.js](https://nodejs.org) — runs the Claude ACP bridge via `npx`
- [claude](https://docs.anthropic.com/en/docs/claude-code) — authenticated Claude Code CLI (the ACP bridge drives it)

> The ACP bridge defaults to `npx --yes @zed-industries/claude-code-acp`. Override it with `GH_COMMIT_ACP_CMD` if you prefer another bridge.

## Usage

```sh
# Initialize scopes for your repo (uses Claude to analyze structure)
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

1. **`gh commit init`** — A Claude agent (over ACP) analyzes your repo structure and generates scope definitions (e.g., `core → src/`, `docs → docs/, README.md`, `ci → .github/workflows/`)
2. **`gh commit`** — Groups dirty files by scope, generates a commit message per scope via the same Claude agent, and commits each group separately
3. **Auto-refresh** — Whenever your `.gitignore` changes, scopes are automatically regenerated before committing (a content hash of `.gitignore` is tracked per repo)
4. Remaining unscoped files are handled in a final pass
5. Unpushed commits are offered for push

Scopes are stored in a local DuckDB database (`~/.local/share/gh-commit/gh-commit.db`) — no config files in your repo.

### Why ACP?

gh-commit talks to Claude through the [Agent Client Protocol](https://agentclientprotocol.com) — the same protocol editors like Zed use to drive coding agents. One Claude backend handles everything (no separate `mods`/LLM CLI), the bridge process is reused across a run, and each generation runs in its own session so context never bleeds between commits.

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
| `GH_COMMIT_ACP_CMD` | Override the Claude ACP bridge command (default `npx --yes @zed-industries/claude-code-acp`) |
| `GH_COMMIT_ACP_PERMISSION` | ACP permission mode (default `bypassPermissions`) |
| `GH_COMMIT_ACP_TIMEOUT` | Per-prompt timeout in seconds (default `300`) |
| `GH_COMMIT_DEBUG=1` | Show ACP bridge stderr and parse diagnostics |

## Migration

Existing `.github/Repo.toml` or `.github/scopes.json` files are automatically detected and migrated to DuckDB on first run.

## License

MIT

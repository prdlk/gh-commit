#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "duckdb>=1.0.0",
#     "rich>=13.0.0",
#     "questionary>=2.0.0",
# ]
# ///
"""gh-commit - AI-powered scoped git commits with DuckDB storage."""

import hashlib
import json
import os
import re
import subprocess
import sys
import tomllib
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Optional

import duckdb
import questionary
from rich.console import Console
from rich.panel import Panel
from rich.table import Table

console = Console()

# ── Config ────────────────────────────────────────────────────────────────────

DB_DIR = Path(os.environ.get("XDG_DATA_HOME", Path.home() / ".local/share")) / "gh-commit"
DB_PATH = DB_DIR / "gh-commit.db"

MODS_CMD = os.environ.get(
    "GH_COMMIT_MODS_CMD",
    "mods --mcp-disable github --mcp-disable context7 -R write-commit",
).split()

AUTO_CONFIRM = os.environ.get("GH_COMMIT_AUTO", "0") == "1"
AUTO_PUSH = os.environ.get("GH_COMMIT_PUSH", "0") == "1"

VERSION = "1.0.0"


# ── Helpers ───────────────────────────────────────────────────────────────────

@dataclass
class Repo:
    id: int
    path: str
    name: str


def run(cmd: list[str], capture: bool = True, check: bool = True) -> subprocess.CompletedProcess:
    return subprocess.run(cmd, capture_output=capture, text=True, check=check)


def git(*args: str) -> str:
    result = run(["git", *args], check=False)
    return result.stdout.strip() if result.returncode == 0 else ""


def is_git_repo() -> bool:
    return run(["git", "rev-parse", "--is-inside-work-tree"], check=False).returncode == 0


def get_repo_root() -> Optional[Path]:
    root = git("rev-parse", "--show-toplevel")
    return Path(root) if root else None


def confirm(msg: str) -> bool:
    if AUTO_CONFIRM:
        return True
    return questionary.confirm(msg, default=True).ask() or False


def require_git() -> Path:
    """Assert we're in a git repo and return the root path."""
    if not is_git_repo():
        console.print("[red]Not in a git repository[/]")
        sys.exit(1)
    return get_repo_root()


# ── Database ──────────────────────────────────────────────────────────────────

def init_db():
    DB_DIR.mkdir(parents=True, exist_ok=True)
    conn = duckdb.connect(str(DB_PATH))
    conn.execute("""
        CREATE TABLE IF NOT EXISTS repositories (
            id INTEGER PRIMARY KEY,
            path TEXT UNIQUE NOT NULL,
            name TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    """)
    conn.execute("""
        CREATE TABLE IF NOT EXISTS scopes (
            id INTEGER PRIMARY KEY,
            repo_id INTEGER NOT NULL,
            name TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            UNIQUE(repo_id, name)
        )
    """)
    conn.execute("""
        CREATE TABLE IF NOT EXISTS scope_paths (
            id INTEGER PRIMARY KEY,
            scope_id INTEGER NOT NULL,
            path TEXT NOT NULL,
            UNIQUE(scope_id, path)
        )
    """)
    conn.execute("""
        CREATE TABLE IF NOT EXISTS github_labels (
            id INTEGER PRIMARY KEY,
            scope_id INTEGER NOT NULL,
            label_name TEXT NOT NULL,
            color TEXT,
            synced_at TIMESTAMP,
            UNIQUE(scope_id)
        )
    """)
    conn.close()


def get_db() -> duckdb.DuckDBPyConnection:
    return duckdb.connect(str(DB_PATH))


def repo_has_scopes(repo_path: str) -> bool:
    conn = get_db()
    result = conn.execute("""
        SELECT COUNT(*) FROM scopes s
        JOIN repositories r ON s.repo_id = r.id
        WHERE r.path = ?
    """, [repo_path]).fetchone()
    conn.close()
    return result[0] > 0 if result else False


def get_repo_scopes(repo_path: str) -> dict[str, list[str]]:
    conn = get_db()
    result = conn.execute("""
        SELECT s.name, sp.path
        FROM scopes s
        JOIN repositories r ON s.repo_id = r.id
        JOIN scope_paths sp ON sp.scope_id = s.id
        WHERE r.path = ?
        ORDER BY s.name, sp.path
    """, [repo_path]).fetchall()
    conn.close()
    scopes: dict[str, list[str]] = {}
    for scope_name, path in result:
        scopes.setdefault(scope_name, []).append(path)
    return scopes


def _cascade_delete_repo(conn, repo_id: int):
    """Delete all child records for a repository."""
    scope_ids = conn.execute("SELECT id FROM scopes WHERE repo_id = ?", [repo_id]).fetchall()
    for (scope_id,) in scope_ids:
        conn.execute("DELETE FROM scope_paths WHERE scope_id = ?", [scope_id])
        conn.execute("DELETE FROM github_labels WHERE scope_id = ?", [scope_id])
    conn.execute("DELETE FROM scopes WHERE repo_id = ?", [repo_id])


def save_scopes(repo_path: str, repo_name: str, scopes: dict[str, list[str]]):
    conn = get_db()
    result = conn.execute("SELECT id FROM repositories WHERE path = ?", [repo_path]).fetchone()
    if result:
        repo_id = result[0]
        _cascade_delete_repo(conn, repo_id)
    else:
        conn.execute("INSERT INTO repositories (path, name) VALUES (?, ?)", [repo_path, repo_name])
        repo_id = conn.execute("SELECT id FROM repositories WHERE path = ?", [repo_path]).fetchone()[0]

    for scope_name, paths in scopes.items():
        conn.execute("INSERT INTO scopes (repo_id, name) VALUES (?, ?)", [repo_id, scope_name])
        scope_id = conn.execute(
            "SELECT id FROM scopes WHERE repo_id = ? AND name = ?",
            [repo_id, scope_name],
        ).fetchone()[0]
        for path in (paths if isinstance(paths, list) else [paths]):
            conn.execute("INSERT INTO scope_paths (scope_id, path) VALUES (?, ?)", [scope_id, path])

    conn.execute("UPDATE repositories SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", [repo_id])
    conn.close()


def list_repos() -> list[tuple]:
    conn = get_db()
    result = conn.execute("""
        SELECT r.name, r.path, COUNT(DISTINCT s.id) as scope_count, r.updated_at
        FROM repositories r
        LEFT JOIN scopes s ON s.repo_id = r.id
        GROUP BY r.id, r.name, r.path, r.updated_at
        ORDER BY r.updated_at DESC
    """).fetchall()
    conn.close()
    return result


def delete_repo(repo_path: str):
    conn = get_db()
    result = conn.execute("SELECT id FROM repositories WHERE path = ?", [repo_path]).fetchone()
    if result:
        repo_id = result[0]
        _cascade_delete_repo(conn, repo_id)
        conn.execute("DELETE FROM repositories WHERE id = ?", [repo_id])
    conn.close()


# ── Migration ─────────────────────────────────────────────────────────────────

def migrate_toml(repo_path: Path, repo_name: str, toml_path: Path) -> bool:
    console.print("[yellow]↻ Migrating .github/Repo.toml → DuckDB[/]")
    try:
        data = tomllib.loads(toml_path.read_text())
        save_scopes(str(repo_path), repo_name, data.get("scopes", {}))
        backup = toml_path.with_suffix(f".toml.migrated.{datetime.now():%Y%m%d_%H%M%S}")
        toml_path.rename(backup)
        console.print(f"[dim]  Archived: {backup}[/]")
        return True
    except Exception as e:
        console.print(f"[red]Migration failed: {e}[/]")
        return False


def migrate_json(repo_path: Path, repo_name: str, json_path: Path) -> bool:
    console.print("[yellow]↻ Migrating .github/scopes.json → DuckDB[/]")
    try:
        data = json.loads(json_path.read_text())
        scopes: dict[str, list[str]] = {}
        for item in data:
            scopes.setdefault(item["scope"], []).append(item["path"])
        save_scopes(str(repo_path), repo_name, scopes)
        backup = json_path.with_suffix(f".json.migrated.{datetime.now():%Y%m%d_%H%M%S}")
        json_path.rename(backup)
        console.print(f"[dim]  Archived: {backup}[/]")
        return True
    except Exception as e:
        console.print(f"[red]Migration failed: {e}[/]")
        return False


def auto_migrate(repo_path: Path) -> bool:
    repo_name = repo_path.name
    toml_path = repo_path / ".github" / "Repo.toml"
    if toml_path.exists():
        return migrate_toml(repo_path, repo_name, toml_path)
    json_path = repo_path / ".github" / "scopes.json"
    if json_path.exists():
        return migrate_json(repo_path, repo_name, json_path)
    return False


# ── Diff filtering ────────────────────────────────────────────────────────────

LOCK_PATTERN = re.compile(
    r"(package-lock\.json|yarn\.lock|pnpm-lock\.yaml|bun\.lockb|"
    r"go\.sum|go\.mod|Cargo\.lock|poetry\.lock|composer\.lock|Gemfile\.lock|"
    r".*\.min\.(js|css)|.*\.bundle\.js|.*\.map|"
    r"dist/.*|build/.*|\.next/.*|node_modules/.*|vendor/.*|__pycache__/.*|\.pyc$|target/.*)"
)

MAX_DIFF_LINES = 200
MAX_JSON_LINES = 50


def filter_diff(diff: str) -> str:
    lines = []
    in_filtered = False
    in_json = False
    line_count = 0
    json_count = 0

    for line in diff.split("\n"):
        if line.startswith("diff --git"):
            line_count = 0
            json_count = 0
            match = re.search(r"b/([^ ]+)", line)
            filename = match.group(1) if match else ""

            if LOCK_PATTERN.search(filename):
                in_filtered, in_json = True, False
                lines.append(line)
                continue
            elif filename.endswith(".json"):
                in_filtered, in_json = False, True
                lines.append(line)
                continue
            else:
                in_filtered, in_json = False, False

        if in_filtered:
            if re.match(r"^(index|---|\+\+\+|@@)", line):
                lines.append(line)
                if line.startswith("@@"):
                    lines.append("[Generated/lock file - content filtered]")
                    in_filtered = False
            continue

        if in_json:
            if re.match(r"^(index|---|\+\+\+|@@)", line):
                lines.append(line)
                continue
            if line.startswith(("+", "-")):
                json_count += 1
                if json_count <= MAX_JSON_LINES:
                    lines.append(line)
                elif json_count == MAX_JSON_LINES + 1:
                    lines.append(f"[... JSON truncated after {MAX_JSON_LINES} lines ...]")
            else:
                lines.append(line)
            continue

        if line_count < MAX_DIFF_LINES:
            lines.append(line)
            line_count += 1
        elif line_count == MAX_DIFF_LINES:
            lines.append(f"[... truncated after {MAX_DIFF_LINES} lines ...]")
            line_count += 1

    return "\n".join(lines)


# ── AI integration ────────────────────────────────────────────────────────────

def generate_commit_message(diff: str, scope: Optional[str] = None) -> Optional[str]:
    filtered = filter_diff(diff)
    context = f"### Git Diff\n{filtered}"
    if scope:
        context += f"\n\n### Scope\n{scope}"
    try:
        result = subprocess.run(
            MODS_CMD, input=context, capture_output=True, text=True, check=True,
        )
        return result.stdout.strip()
    except subprocess.CalledProcessError as e:
        console.print(f"[red]Failed to generate commit message: {e}[/]")
        return None


def generate_scopes_with_claude(repo_path: Path, existing_toml: Optional[str] = None) -> Optional[str]:
    if existing_toml:
        prompt = f"""Analyze this git repository and UPDATE the existing scopes configuration.

Current scopes:
{existing_toml}

Your task:
1. Review the current scopes and their paths
2. Analyze the current repository directory structure
3. Update the scopes to reflect any new directories or structural changes
4. Keep existing scopes that are still relevant
5. Remove scopes for paths that no longer exist
6. Add new scopes for new directories or logical areas

Output ONLY the updated TOML content (no markdown, no explanation, just the raw TOML)."""
    else:
        prompt = """Analyze this git repository and generate a TOML scopes configuration.

The output should:
1. Have a [scopes] section
2. Map logical project areas to arrays of directory paths
3. Each scope can have MULTIPLE paths
4. Use scope names that work well in conventional commits (e.g., 'docs', 'tests', 'config', 'ci', 'scripts')
5. Cover the main directories in the repository
6. Group related paths under the same scope when appropriate

Example format:
[scopes]
ci = [".github/workflows"]
config = [".github", ".config", "package.json"]
docs = ["docs", "README.md"]
tests = ["tests", "__tests__"]
scripts = ["scripts", "bin"]
core = ["src"]

Please analyze the repository structure and output ONLY the TOML content (no markdown, no explanation, just the raw TOML)."""

    try:
        with console.status("[magenta]Analyzing repository with Claude...[/]"):
            result = subprocess.run(
                ["claude", "-p"], input=prompt,
                capture_output=True, text=True, check=True, cwd=repo_path,
            )
        output = result.stdout.strip()
        if "[scopes]" in output:
            output = output[output.find("[scopes]"):]
        if "```" in output:
            output = output[: output.find("```")]
        return output.strip()
    except subprocess.CalledProcessError as e:
        console.print(f"[red]Claude failed: {e.stderr}[/]")
        return None
    except FileNotFoundError:
        console.print("[red]Error: 'claude' command not found. Install Claude Code:[/]")
        console.print("[cyan]  npm install -g @anthropic-ai/claude-code[/]")
        return None


def parse_toml_scopes(toml_str: str) -> Optional[dict[str, list[str]]]:
    try:
        return tomllib.loads(toml_str).get("scopes", {})
    except Exception:
        return None


def scopes_to_toml(scopes: dict[str, list[str]]) -> str:
    lines = ["[scopes]"]
    for name, paths in sorted(scopes.items()):
        paths_str = ", ".join(f'"{p}"' for p in paths)
        lines.append(f'{name} = [{paths_str}]')
    return "\n".join(lines)


def display_scopes(scopes: dict[str, list[str]]):
    for name, paths in sorted(scopes.items()):
        console.print(f"  [cyan]•[/] [bold]{name}[/]: {', '.join(paths)}")


# ── Git operations ────────────────────────────────────────────────────────────

def get_changed_files() -> list[str]:
    staged = git("diff", "--cached", "--name-only").split("\n")
    unstaged = git("diff", "--name-only").split("\n")
    untracked = git("ls-files", "--others", "--exclude-standard").split("\n")
    return sorted(set(f for f in staged + unstaged + untracked if f))


def get_files_in_scope(files: list[str], paths: list[str]) -> list[str]:
    matched = []
    for f in files:
        for path in paths:
            if f.startswith(path):
                matched.append(f)
                break
    return matched


def stage_files(files: list[str]):
    if files:
        subprocess.run(["git", "add", *files], check=True)


def reset_staging():
    subprocess.run(["git", "reset", "HEAD", "--", "."], capture_output=True)


def do_commit(message: str):
    subprocess.run(["git", "commit", "-m", message], check=True)


def get_unpushed_commits() -> list[str]:
    output = git("log", "--branches", "--not", "--remotes", "--oneline")
    return [line for line in output.split("\n") if line]


def push_to_origin():
    branch = git("branch", "--show-current")
    with console.status(f"[magenta]Pushing to origin/{branch}...[/]"):
        subprocess.run(["git", "push", "origin", branch], check=True)
    console.print(f"[green]✓ Pushed to origin/{branch}[/]")


# ── Commands ──────────────────────────────────────────────────────────────────

def cmd_version():
    print(f"gh-commit {VERSION}")


def cmd_help():
    console.print(f"[magenta bold]gh commit[/] [dim]v{VERSION}[/] — AI-powered scoped git commits\n")
    console.print("[cyan]Usage:[/]")
    console.print("  gh commit                  Commit changes grouped by scope")
    console.print("  gh commit --auto           Auto-confirm all prompts")
    console.print("  gh commit --push           Auto-push after committing")
    console.print("  gh commit --auto --push    Both")
    console.print("  gh commit init             Generate scopes for this repo")
    console.print("  gh commit refresh          Update scopes from current structure")
    console.print("  gh commit sync             Sync scopes → GitHub labels")
    console.print("  gh commit list             List all configured repositories")
    console.print("  gh commit remove           Remove current repo from database")
    console.print("  gh commit db-path          Print database file path")
    console.print("  gh commit version          Print version")
    console.print("  gh commit help             Show this help\n")
    console.print("[cyan]Database:[/]")
    console.print(f"  {DB_PATH}\n")
    console.print("[cyan]Environment:[/]")
    console.print("  GH_COMMIT_AUTO=1           Skip all confirmation prompts")
    console.print("  GH_COMMIT_PUSH=1           Auto-push after commits")
    console.print("  GH_COMMIT_MODS_CMD=...     Override mods command\n")
    console.print("[cyan]Migration:[/]")
    console.print("  Existing .github/Repo.toml or scopes.json are auto-migrated on first run")


def cmd_db_path():
    print(DB_PATH)


def cmd_list():
    console.print("[magenta bold]Repositories[/]\n")
    repos = list_repos()
    if not repos:
        console.print("[dim]No repositories configured yet[/]")
        console.print("\n[cyan]Run 'gh commit init' in a git repository to get started[/]")
        return
    table = Table(show_header=True)
    table.add_column("Name", style="bold")
    table.add_column("Path")
    table.add_column("Scopes", justify="right")
    for name, path, scope_count, _ in repos:
        table.add_row(name, path, str(scope_count))
    console.print(table)


def cmd_remove():
    repo_path = require_git()
    if not repo_has_scopes(str(repo_path)):
        console.print(f"[dim]Repository not in database: {repo_path.name}[/]")
        return 0
    if confirm(f"Remove {repo_path.name} from database?"):
        delete_repo(str(repo_path))
        console.print(f"[green]✓ Removed {repo_path.name}[/]")
    else:
        console.print("[dim]Cancelled[/]")
    return 0


def cmd_init():
    repo_path = require_git()
    repo_name = repo_path.name

    # Check for legacy files
    toml_path = repo_path / ".github" / "Repo.toml"
    json_path = repo_path / ".github" / "scopes.json"
    if toml_path.exists() or json_path.exists():
        console.print("[yellow]Found legacy config file(s)[/]")
        if confirm("Migrate to DuckDB?"):
            if auto_migrate(repo_path):
                console.print("[green]✓ Migration complete[/]")
                return 0

    if repo_has_scopes(str(repo_path)):
        console.print("[yellow]⚠ Repository already configured[/]")
        if not confirm("Overwrite existing scopes?"):
            console.print("[dim]Cancelled[/]")
            return 0

    console.print(f"[magenta bold]Generating scopes for {repo_name}...[/]\n")
    toml_output = generate_scopes_with_claude(repo_path)
    if not toml_output:
        return 1

    scopes = parse_toml_scopes(toml_output)
    if not scopes:
        console.print("[red]Error: Generated output is not valid TOML[/]")
        console.print(f"[dim]{toml_output}[/]")
        return 1

    save_scopes(str(repo_path), repo_name, scopes)
    console.print("[green]✓ Saved scopes to database[/]\n")
    console.print("[magenta]Generated scopes:[/]")
    display_scopes(scopes)
    console.print("\n[dim]Run 'gh commit' to use these scopes[/]")
    return 0


def cmd_refresh():
    repo_path = require_git()
    repo_name = repo_path.name

    if not repo_has_scopes(str(repo_path)):
        console.print("[red]Repository not configured — run 'gh commit init' first[/]")
        return 1

    existing_scopes = get_repo_scopes(str(repo_path))
    console.print(f"[magenta bold]Refreshing scopes for {repo_name}...[/]\n")
    console.print("[cyan]Current scopes:[/]")
    display_scopes(existing_scopes)
    console.print()

    if not confirm("Refresh scopes based on current structure?"):
        console.print("[dim]Cancelled[/]")
        return 0

    toml_output = generate_scopes_with_claude(repo_path, scopes_to_toml(existing_scopes))
    if not toml_output:
        return 1

    scopes = parse_toml_scopes(toml_output)
    if not scopes:
        console.print("[red]Error: Generated output is not valid TOML[/]")
        return 1

    console.print("\n[green]✓ Generated updated scopes[/]\n")
    console.print("[magenta]Updated scopes:[/]")
    display_scopes(scopes)
    console.print()

    if confirm("Apply these changes?"):
        save_scopes(str(repo_path), repo_name, scopes)
        console.print("[green]✓ Updated scopes[/]")
    else:
        console.print("[dim]Changes not applied[/]")
    return 0


def cmd_sync():
    repo_path = require_git()

    if not repo_has_scopes(str(repo_path)):
        console.print("[red]Repository not configured — run 'gh commit init' first[/]")
        return 1

    if subprocess.run(["which", "gh"], capture_output=True).returncode != 0:
        console.print("[red]Error: 'gh' command not found[/]")
        return 1
    if subprocess.run(["gh", "repo", "view"], capture_output=True).returncode != 0:
        console.print("[red]Error: Not a GitHub repository or not authenticated[/]")
        return 1

    console.print("[magenta bold]Syncing scopes → GitHub labels...[/]\n")
    scopes = get_repo_scopes(str(repo_path))
    created = updated = failed = 0

    for scope_name, paths in scopes.items():
        desc = f"Changes to: {', '.join(paths)}"
        color = hashlib.md5(scope_name.encode()).hexdigest()[:6]

        result = subprocess.run(
            ["gh", "label", "create", scope_name, "--description", desc, "--color", color],
            capture_output=True,
        )
        if result.returncode == 0:
            console.print(f"  [green]✓[/] Created: {scope_name}")
            created += 1
        else:
            result = subprocess.run(
                ["gh", "label", "edit", scope_name, "--description", desc, "--color", color],
                capture_output=True,
            )
            if result.returncode == 0:
                console.print(f"  [yellow]↻[/] Updated: {scope_name}")
                updated += 1
            else:
                console.print(f"  [red]✗[/] Failed: {scope_name}")
                failed += 1

    console.print(f"\n[green bold]Sync complete![/] Created: {created} | Updated: {updated} | Failed: {failed}")
    return 0


def cmd_commit():
    repo_path = require_git()

    auto_migrate(repo_path)

    if not repo_has_scopes(str(repo_path)):
        console.print("\n[yellow bold]⚠ No scopes configured for this repository[/]\n")
        console.print("[cyan]gh-commit organizes commits by project areas (scopes).[/]\n")
        if confirm("Generate scopes now using Claude?"):
            return cmd_init()
        else:
            console.print("\n[dim]Run 'gh commit init' to configure scopes[/]")
            return 1

    console.print("[magenta]Finding scopes with changes...[/]")
    changed_files = get_changed_files()
    scopes = get_repo_scopes(str(repo_path))

    scopes_with_changes = [
        name for name, paths in scopes.items()
        if get_files_in_scope(changed_files, paths)
    ]

    if not scopes_with_changes:
        console.print("[dim]No scoped changes found[/]")
    else:
        console.print(f"[cyan]Scopes with changes: {' '.join(scopes_with_changes)}[/]\n")

    # Process each scope
    for scope in scopes_with_changes:
        console.print(f"[magenta bold]Processing scope: {scope}[/]")
        paths = scopes[scope]
        console.print(f"[cyan]  Paths: {', '.join(paths)}[/]")

        reset_staging()

        scope_files = []
        status_output = git("status", "--porcelain")
        for line in status_output.split("\n"):
            if not line:
                continue
            filename = line[3:] if len(line) > 3 else line.split()[-1]
            for path in paths:
                if filename.startswith(path):
                    scope_files.append(filename)
                    break

        if not scope_files:
            console.print("[dim]  No files found in scope paths[/]")
            continue

        console.print("[dim]  Files to stage:[/]")
        for f in scope_files:
            console.print(f"[dim]    {f}[/]")

        stage_files(scope_files)
        diff = git("diff", "--cached")
        if not diff:
            console.print(f"[dim]  No changes to commit for {scope}[/]")
            continue

        with console.status("[magenta]Generating commit message...[/]"):
            message = generate_commit_message(diff, scope)
        if not message:
            continue

        console.print(Panel(message, border_style="magenta"))
        if confirm(f"Commit changes for {scope}?"):
            do_commit(message)
            console.print(f"[green]✓ Committed changes for {scope}[/]")
        else:
            console.print(f"[dim]  Skipped {scope}[/]")
        console.print()

    reset_staging()

    # Remaining files
    remaining = git("status", "--porcelain")
    if remaining:
        console.print("\n[yellow]Processing remaining files outside any scope...[/]")

        tracked_unstaged = git("diff", "--name-only")
        if tracked_unstaged:
            files = [f for f in tracked_unstaged.split("\n") if f]
            console.print("\n[magenta]Tracked unstaged files:[/]")
            for f in files:
                console.print(f"[dim]  {f}[/]")
            if confirm("Commit tracked unstaged files?"):
                stage_files(files)
                diff = git("diff", "--cached")
                with console.status("[magenta]Generating commit message...[/]"):
                    message = generate_commit_message(diff)
                if message:
                    console.print(Panel(message, border_style="magenta"))
                    do_commit(message)
                    console.print("[green]✓ Committed tracked unstaged files[/]")

        untracked = git("ls-files", "--others", "--exclude-standard")
        if untracked:
            files = [f for f in untracked.split("\n") if f]
            console.print("\n[magenta]Untracked files:[/]")
            for f in files:
                console.print(f"[dim]  {f}[/]")
            if confirm("Commit untracked files?"):
                stage_files(files)
                diff = git("diff", "--cached")
                with console.status("[magenta]Generating commit message...[/]"):
                    message = generate_commit_message(diff)
                if message:
                    console.print(Panel(message, border_style="magenta"))
                    do_commit(message)
                    console.print("[green]✓ Committed untracked files[/]")

    # Push
    unpushed = get_unpushed_commits()
    if unpushed:
        console.print("\n[magenta bold]Unpushed Commits[/]")
        console.print(f"[cyan]{len(unpushed)} commit(s) ready to push:[/]\n")
        for line in unpushed:
            parts = line.split(" ", 1)
            hash_val = parts[0]
            msg = parts[1] if len(parts) > 1 else ""
            console.print(f"  [bold]{hash_val}[/] {msg}")
        console.print()
        if AUTO_PUSH or confirm("Push commits to origin?"):
            push_to_origin()
        else:
            console.print("[dim]Skipped push[/]")
    else:
        console.print("\n[dim]No unpushed commits[/]")

    console.print("\n[green bold]✓ Done![/]")
    return 0


# ── Entrypoint ────────────────────────────────────────────────────────────────

COMMANDS = {
    "init": cmd_init,
    "refresh": cmd_refresh,
    "sync": cmd_sync,
    "list": cmd_list,
    "remove": cmd_remove,
    "db-path": cmd_db_path,
    "version": cmd_version,
    "help": cmd_help,
}


def main():
    global AUTO_CONFIRM, AUTO_PUSH

    args = sys.argv[1:]

    # Parse flags
    while args and args[0].startswith("--"):
        flag = args.pop(0)
        if flag == "--auto":
            AUTO_CONFIRM = True
        elif flag == "--push":
            AUTO_PUSH = True
        elif flag in ("--help", "-h"):
            cmd_help()
            return
        elif flag == "--version":
            cmd_version()
            return
        else:
            # Unknown flag — might be a legacy --init style command
            legacy = flag.lstrip("-")
            if legacy in COMMANDS:
                args.insert(0, legacy)
                break
            console.print(f"[red]Unknown flag: {flag}[/]")
            console.print("[dim]Use 'gh commit help' for usage[/]")
            sys.exit(1)

    init_db()

    cmd = args[0] if args else None
    if cmd is None:
        sys.exit(cmd_commit())
    elif cmd in COMMANDS:
        result = COMMANDS[cmd]()
        if isinstance(result, int):
            sys.exit(result)
    else:
        console.print(f"[red]Unknown command: {cmd}[/]")
        console.print("[dim]Use 'gh commit help' for usage[/]")
        sys.exit(1)


if __name__ == "__main__":
    main()

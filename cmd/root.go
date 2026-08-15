// Package cmd wires the cobra CLI: config resolution, database and Ollama
// client lifecycles, and one file per subcommand.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/prdlk/gh-commit/internal/ai"
	"github.com/prdlk/gh-commit/internal/db"
	"github.com/prdlk/gh-commit/internal/gitx"
	"github.com/prdlk/gh-commit/internal/ui"
)

// errAborted signals exit code 1 after the failure has already been printed.
var errAborted = errors.New("aborted")

type config struct {
	host          string
	model         string
	timeout       time.Duration
	auto          bool
	push          bool
	noAutoRefresh bool
	debug         bool
}

var (
	version = "dev"
	cfg     config
	store   *db.Store
	client  *ai.Client

	flagHost  string
	flagModel string
	flagAuto  bool
	flagPush  bool
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string) bool { return os.Getenv(key) == "1" }

func longHelp() string {
	return fmt.Sprintf(`AI-powered scoped git commits driven by a local Ollama model.

Scopes map Conventional Commit scope names to path prefixes. They are
generated from the repository file tree by the model, stored in a local
SQLite database, and auto-regenerate whenever .gitignore changes.
Legacy .github/Repo.toml or scopes.json files are auto-migrated on first run.

Database:
  %s

Environment:
  GH_COMMIT_OLLAMA_HOST      Ollama server URL (default http://localhost:11434)
  GH_COMMIT_MODEL            Model tag (default qwen3.5:2b)
  GH_COMMIT_TIMEOUT          Per-request timeout in seconds (default 120)
  GH_COMMIT_AUTO=1           Skip all confirmation prompts
  GH_COMMIT_PUSH=1           Auto-push after commits
  GH_COMMIT_NO_AUTO_REFRESH=1  Don't auto-regenerate scopes on .gitignore change
  GH_COMMIT_DEBUG=1          Print raw model output on parse failure`, db.Path())
}

var rootCmd = &cobra.Command{
	Use:           "gh-commit",
	Short:         "AI-powered scoped git commits (Ollama)",
	Long:          longHelp(),
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		cfg.host = envOr("GH_COMMIT_OLLAMA_HOST", "http://localhost:11434")
		cfg.model = envOr("GH_COMMIT_MODEL", "qwen3.5:2b")
		if flagHost != "" {
			cfg.host = flagHost
		}
		if flagModel != "" {
			cfg.model = flagModel
		}
		seconds := 120
		if v, err := strconv.Atoi(envOr("GH_COMMIT_TIMEOUT", "120")); err == nil && v > 0 {
			seconds = v
		}
		cfg.timeout = time.Duration(seconds) * time.Second
		cfg.auto = envBool("GH_COMMIT_AUTO") || flagAuto
		cfg.push = envBool("GH_COMMIT_PUSH") || flagPush
		cfg.noAutoRefresh = envBool("GH_COMMIT_NO_AUTO_REFRESH")
		cfg.debug = envBool("GH_COMMIT_DEBUG")
		ui.AutoConfirm = cfg.auto

		switch cmd.Name() {
		case "version", "help":
			return nil
		}
		s, err := db.Open()
		if err != nil {
			return err
		}
		store = s
		if s.HadLegacyDuckDB {
			ui.Warnf("Found a legacy DuckDB database (gh-commit.db) — it cannot be migrated; scopes will regenerate on 'gh commit init'")
		}
		return nil
	},
	RunE: runCommit,
}

// Execute runs the CLI, printing errors in red and exiting 1 on failure.
func Execute(v string) {
	version = v
	rootCmd.Version = v
	rootCmd.SetVersionTemplate("gh-commit {{.Version}}\n")
	err := rootCmd.Execute()
	if store != nil {
		_ = store.Close()
	}
	if err != nil {
		if !errors.Is(err, errAborted) {
			ui.Errorf("%s", err)
		}
		os.Exit(1)
	}
}

// requireGit asserts we're inside a git repo and returns its root path.
func requireGit() (string, error) {
	if !gitx.IsRepo() {
		ui.Errorf("Not in a git repository")
		return "", errAborted
	}
	return gitx.RepoRoot(), nil
}

// ensureAI builds the Ollama client and verifies server + model availability.
func ensureAI() error {
	if client != nil {
		return nil
	}
	c := ai.New(cfg.host, cfg.model, cfg.timeout)
	if err := c.EnsureReady(); err != nil {
		return err
	}
	client = c
	return nil
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagAuto, "auto", false, "skip all confirmation prompts")
	rootCmd.PersistentFlags().BoolVar(&flagPush, "push", false, "auto-push after committing")
	rootCmd.PersistentFlags().StringVar(&flagModel, "model", "", "override the Ollama model tag")
	rootCmd.PersistentFlags().StringVar(&flagHost, "host", "", "override the Ollama server URL")
}

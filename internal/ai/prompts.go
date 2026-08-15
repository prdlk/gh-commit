package ai

import (
	"encoding/json"
	"strings"
)

// Prompt text is ported unchanged from the Python tool, plus one
// "Do not think" line per prompt as cheap insurance on small models.

const commitPrompt = `You are an expert conventional commit message writer.
Use one of these commit types: feat, fix, docs, style, refactor, breaking, test, perf, build, ci, chore, init.
Determine a precise commit message from the provided diff and scope.
The commit message must follow this format: <type>(<scope>): <description>
Examples:
- fix(core): resolve consensus timeout during high load
- feat(did): add WebAuthn biometric authentication support
- refactor(hway): extract Redis connection pooling to shared utility
- init(browser): setup client package for web API-specific logic
- perf(vault): optimize IPFS chunk size for large file uploads
- ci(actions): update GitHub Actions pipeline for parallel module testing
- docs(dwn): clarify data retention policies in API reference
- test(svc): add integration tests for domain verification flow
- breaking(ui): rename Button prop 'type' to 'variant'
- refactor(sdk): migrate off wallet implementation in favor of @sonr.io/enclave
- feat(react): create Enclave stateful hooks
Never explain anything. Return only the commit message.
Do not think. Answer immediately.

{scope_hint}### Git Diff
{diff}
`

const scopeInstructions = `You are a repository scope identifier for Conventional Commits.
You receive a repository file tree, one repo-relative path per line.
Map logical project areas to the repo-relative path prefixes (directories or files) they cover.
Scope names must read well in Conventional Commits, e.g. core, api, ui, cli, docs, tests, ci, config, scripts, deps.
Each scope maps to a JSON array of repo-relative path prefixes.
Group related paths together and cover the meaningful source areas.
Never create scopes for generated, vendored, or build-output paths.
If existing scopes are provided, keep the ones that still apply, drop scopes whose paths no longer exist, and add scopes for new areas.
Never explain anything or print your thoughts. Never include Markdown code blocks.
Only return a single JSON object of the form {"scope": ["path", ...], ...}.
Do not think. Answer immediately.
`

const scopePromptNew = scopeInstructions + `
Repository file tree (one repo-relative path per line):

{filetree}
`

const scopePromptUpdate = scopeInstructions + `
Existing scopes (JSON):
{existing}

The repository structure may have changed (e.g. its .gitignore was edited).
Updated repository file tree (one repo-relative path per line):

{filetree}
`

// buildCommitPrompt fills the commit prompt with an optional scope hint and
// the (already filtered) diff.
func buildCommitPrompt(filteredDiff, scope string) string {
	hint := ""
	if scope != "" {
		hint = "Use this scope: " + scope + "\n"
	}
	return strings.NewReplacer("{scope_hint}", hint, "{diff}", filteredDiff).Replace(commitPrompt)
}

// buildScopePrompt fills the scope prompt; existing may be nil for first-time
// generation.
func buildScopePrompt(filetree string, existing map[string][]string) string {
	if len(existing) == 0 {
		return strings.Replace(scopePromptNew, "{filetree}", filetree, 1)
	}
	blob, _ := json.MarshalIndent(existing, "", "  ")
	return strings.NewReplacer(
		"{existing}", string(blob),
		"{filetree}", filetree,
	).Replace(scopePromptUpdate)
}

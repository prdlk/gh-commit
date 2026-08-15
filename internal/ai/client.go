// Package ai talks to a local Ollama server. The rozoomcool SDK is used for
// client construction and model pulls; generation goes through a raw
// /api/generate POST because the SDK's Generate cannot pass think, options,
// or keep_alive — the levers this tool depends on for speed.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ollama "github.com/rozoomcool/go-ollama-sdk"

	"github.com/prdlk/gh-commit/internal/diff"
	"github.com/prdlk/gh-commit/internal/ui"
)

// Generation profiles: small budget for one-line commit messages, larger
// budget and context for scope JSON over big file trees.
const (
	commitNumPredict = 96
	commitNumCtx     = 8192
	scopeNumPredict  = 1024
	scopeNumCtx      = 16384
	keepAlive        = "10m"
	temperature      = 0.2
	topP             = 0.9
)

// Client is a speed-tuned Ollama client for one model.
type Client struct {
	host    string
	model   string
	timeout time.Duration
	http    *http.Client
	sdk     *ollama.OllamaClient
}

// New builds a client for host/model with a per-request timeout.
func New(host, model string, timeout time.Duration) *Client {
	host = strings.TrimRight(host, "/")
	return &Client{
		host:    host,
		model:   model,
		timeout: timeout,
		http:    &http.Client{Timeout: timeout},
		sdk:     ollama.NewClient(host),
	}
}

// Model returns the configured model tag.
func (c *Client) Model() string { return c.model }

type generateOptions struct {
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	NumPredict  int     `json:"num_predict"`
	NumCtx      int     `json:"num_ctx"`
}

type generateRequest struct {
	Model     string          `json:"model"`
	Prompt    string          `json:"prompt"`
	Stream    bool            `json:"stream"`
	Think     bool            `json:"think"`
	KeepAlive string          `json:"keep_alive"`
	Options   generateOptions `json:"options"`
}

type generateResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

// generate POSTs a raw /api/generate request with the speed profile applied.
func (c *Client) generate(prompt string, numPredict, numCtx int) (string, error) {
	body, err := json.Marshal(generateRequest{
		Model:     c.model,
		Prompt:    prompt,
		Stream:    false,
		Think:     false,
		KeepAlive: keepAlive,
		Options: generateOptions{
			Temperature: temperature,
			TopP:        topP,
			NumPredict:  numPredict,
			NumCtx:      numCtx,
		},
	})
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out generateResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("ollama returned unexpected payload: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if out.Error != "" {
			return "", fmt.Errorf("ollama: %s", out.Error)
		}
		return "", fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}
	// think:false is sent, but strip any reasoning block as defense in depth.
	return stripThink(out.Response), nil
}

// CommitMessage filters diff, prompts the model, and returns a cleaned
// commit message ("" when the model produced nothing usable).
func (c *Client) CommitMessage(rawDiff, scope string) (string, error) {
	prompt := buildCommitPrompt(diff.Filter(rawDiff), scope)
	out, err := c.generate(prompt, commitNumPredict, commitNumCtx)
	if err != nil {
		return "", err
	}
	return CleanCommitMessage(out), nil
}

// ScopesRaw prompts the model for a scope mapping and returns the raw
// response text (callers parse it so they can show diagnostics on failure).
func (c *Client) ScopesRaw(filetree string, existing map[string][]string) (string, error) {
	return c.generate(buildScopePrompt(filetree, existing), scopeNumPredict, scopeNumCtx)
}

type tagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// EnsureReady verifies the Ollama server is answering and the model is
// available locally, offering to pull it when missing.
func (c *Client) EnsureReady() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.host+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("the Ollama server is not reachable at %s — start it with 'ollama serve'", c.host)
	}
	defer func() { _ = resp.Body.Close() }()

	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("the Ollama server at %s gave an unexpected response — start it with 'ollama serve'", c.host)
	}
	for _, m := range tags.Models {
		if m.Name == c.model || (!strings.Contains(c.model, ":") && m.Name == c.model+":latest") {
			return nil
		}
	}

	ui.Warnf("Model %s is not available locally", c.model)
	if !ui.Confirm(fmt.Sprintf("Pull %s now?", c.model)) {
		return fmt.Errorf("model %s is not available — run 'ollama pull %s'", c.model, c.model)
	}
	last := ""
	if err := c.sdk.PullModel(c.model, func(status string) {
		if status != "" && status != last {
			ui.Dimf("  %s", status)
			last = status
		}
	}); err != nil {
		return fmt.Errorf("pulling %s: %w", c.model, err)
	}
	ui.Successf("✓ Pulled %s", c.model)
	return nil
}

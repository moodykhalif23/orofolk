package ai

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// ClaudeNarrator writes the executive briefing with the Anthropic Messages API.
// It is selected only when AI_PROVIDER=claude and a key is set; otherwise the
// deterministic narrator is used. The brief is the grounding — the model is told
// to use only the facts it contains.
type ClaudeNarrator struct {
	apiKey string
	model  string
	client *http.Client
}

func NewClaudeNarrator(apiKey, model string) *ClaudeNarrator {
	if model == "" {
		model = "claude-opus-4-8"
	}
	return &ClaudeNarrator{apiKey: apiKey, model: model, client: &http.Client{Timeout: 45 * time.Second}}
}

func (ClaudeNarrator) Name() string { return "claude" }

func (c *ClaudeNarrator) Narrate(ctx context.Context, brief InsightBrief) (string, error) {
	resp, err := anthropicMessages(ctx, c.client, c.apiKey, map[string]any{
		"model":      c.model,
		"max_tokens": 1024,
		"system":     narratorSystem,
		"messages":   []map[string]any{{"role": "user", "content": briefToFacts(brief)}},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(firstText(resp.Content)), nil
}

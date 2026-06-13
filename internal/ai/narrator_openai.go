package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAINarrator writes the executive briefing through any OpenAI-compatible
// chat-completions endpoint (Groq, Together, Ollama, vLLM, …). Selected only when
// AI_PROVIDER=openai and a key is set; otherwise the deterministic narrator runs.
type OpenAINarrator struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewOpenAINarrator(baseURL, apiKey, model string) *OpenAINarrator {
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	if model == "" {
		model = "llama-3.3-70b-versatile"
	}
	return &OpenAINarrator{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 45 * time.Second},
	}
}

func (OpenAINarrator) Name() string { return "openai" }

func (p *OpenAINarrator) Narrate(ctx context.Context, brief InsightBrief) (string, error) {
	resp, err := openAIChatCompletions(ctx, p.client, p.baseURL, p.apiKey, map[string]any{
		"model":      p.model,
		"max_tokens": 1024,
		"messages": []map[string]any{
			{"role": "system", "content": narratorSystem},
			{"role": "user", "content": briefToFacts(brief)},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("openai api: empty completion")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

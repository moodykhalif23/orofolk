package ai

import (
	"context"
	"strings"
)

// DeterministicProvider is the default decision engine: it asks each available
// tool whether it matches the message (the tool owns its intent keywords + slot
// extraction), in registry order, and runs the first that does. It performs no
// external calls and is fully reproducible — the same message + tools always
// yields the same decision. Composition simply returns the tool's own summary.
type DeterministicProvider struct{}

func NewDeterministicProvider() *DeterministicProvider { return &DeterministicProvider{} }

func (DeterministicProvider) Name() string { return "deterministic" }

func (DeterministicProvider) Decide(_ context.Context, msg string, _ []Turn, tools []Tool) (Decision, error) {
	for _, t := range tools {
		if args, ok := t.Match(msg); ok {
			return Decision{Tool: t.Name(), Args: args}, nil
		}
	}
	return Decision{}, nil // no match → agent falls back to help text
}

func (DeterministicProvider) Compose(_ context.Context, _ string, _ Tool, result ToolResult, _ []Turn) (string, error) {
	return result.Summary, nil
}

// ---- shared slot helpers for tool Match implementations ------------------

// containsAny reports whether the lowercased message contains any of the terms.
func containsAny(msg string, terms ...string) bool {
	m := strings.ToLower(msg)
	for _, t := range terms {
		if strings.Contains(m, t) {
			return true
		}
	}
	return false
}

// firstUUIDPrefix extracts the first token that looks like a (possibly truncated)
// public id: a hex/dash run of 8+ chars. Returns "" when none is present. Used by
// tools that look up an entity by its public_id or an 8-char prefix shown in UIs.
func firstToken(msg string, minLen int) string {
	for _, raw := range strings.FieldsFunc(msg, func(r rune) bool {
		return r == ' ' || r == ',' || r == '#' || r == '\n' || r == '\t' || r == '"' || r == '\''
	}) {
		tok := strings.TrimRight(raw, ".")
		if len(tok) >= minLen && isHexDash(tok) {
			return tok
		}
	}
	return ""
}

func isHexDash(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		case r == '-':
		default:
			return false
		}
	}
	return len(s) > 0
}

package ai

import (
	"context"
	"strings"
)

// InsightBrief is the structured set of facts the Narrator turns into an
// executive briefing. It is plain data (no DB types), so the ai package stays a
// leaf and internal/insights can build a brief from its own Snapshot and pass it
// in without an import cycle. The narrator is instructed to use ONLY these facts,
// so a generated narrative can never drift from the numbers the engine computed.
type InsightBrief struct {
	// Period is a human label for the window, e.g. "week of Jun 6–Jun 12, 2026".
	Period string
	// Currency is the org's reporting currency (a display hint for the narrator).
	Currency string
	// Headlines are pre-formatted metric lines, e.g.
	// "Revenue 124,500 (+8.2% vs prior week)", "Orders 312 (-3%)".
	Headlines []string
	// Anomalies are the ranked signals the deterministic detector surfaced.
	Anomalies []BriefAnomaly
}

// BriefAnomaly is one detected signal, already ranked and explained by the
// engine. Severity is "critical" | "warn" | "info".
type BriefAnomaly struct {
	Severity       string
	Title          string
	Detail         string
	Recommendation string
}

// Narrator writes a short executive briefing from a brief. ClaudeNarrator and
// OpenAINarrator use a language model; DeterministicNarrator composes a sensible
// briefing offline and is the default when no API key is configured (mirrors the
// PageDesigner and chat-Provider splits).
//
// This is AI woven into the daily workflow rather than a chatbot: the system
// invokes the narrator on a schedule and the briefing lands on the dashboard and
// in the inbox — the operator never has to ask.
type Narrator interface {
	Name() string
	Narrate(ctx context.Context, brief InsightBrief) (string, error)
}

// narratorSystem is the shared system prompt for the language-model narrators.
// The structured brief — not the prompt — is what bounds the content: the model
// is told to use only the facts given and to invent nothing.
const narratorSystem = "You are a sharp operations and finance analyst writing the weekly executive " +
	"briefing for the operator of a B2B commerce business. You are given a set of FACTS (period metrics " +
	"and detected signals). Write a concise briefing of 2–4 short paragraphs:\n" +
	"1. Lead with the headline — revenue and orders for the period and the trend vs the prior period.\n" +
	"2. Then the signals that matter most (risks first), saying plainly what each means for the business.\n" +
	"3. Close with 1–2 concrete recommended actions drawn from the signals.\n" +
	"Rules: use ONLY the facts provided — never invent or estimate numbers. Be direct and confident, " +
	"no hedging, no filler, no markdown headings or bullet symbols. Write for a busy executive."

// briefToFacts renders a brief into the plain FACTS block handed to a
// language-model narrator. Shared by ClaudeNarrator and OpenAINarrator.
func briefToFacts(b InsightBrief) string {
	var sb strings.Builder
	sb.WriteString("Period: ")
	sb.WriteString(b.Period)
	if b.Currency != "" {
		sb.WriteString("\nReporting currency: ")
		sb.WriteString(b.Currency)
	}
	sb.WriteString("\n\nMetrics:")
	if len(b.Headlines) == 0 {
		sb.WriteString("\n- (no sales in this period)")
	}
	for _, h := range b.Headlines {
		sb.WriteString("\n- ")
		sb.WriteString(h)
	}
	sb.WriteString("\n\nSignals detected:")
	if len(b.Anomalies) == 0 {
		sb.WriteString("\n- none (no risks flagged)")
	}
	for _, a := range b.Anomalies {
		sb.WriteString("\n- [")
		sb.WriteString(a.Severity)
		sb.WriteString("] ")
		sb.WriteString(a.Title)
		if a.Detail != "" {
			sb.WriteString(" — ")
			sb.WriteString(a.Detail)
		}
		if a.Recommendation != "" {
			sb.WriteString(" (suggested: ")
			sb.WriteString(a.Recommendation)
			sb.WriteString(")")
		}
	}
	return sb.String()
}

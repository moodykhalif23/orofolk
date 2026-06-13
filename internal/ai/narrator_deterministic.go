package ai

import (
	"context"
	"fmt"
	"strings"
)

// DeterministicNarrator composes an executive briefing from the structured brief
// with no external calls — fully reproducible and the default when no API key is
// configured. It is intentionally plain-spoken: a headline metrics sentence, the
// risks grouped by severity, and the single most important recommended action.
type DeterministicNarrator struct{}

func NewDeterministicNarrator() *DeterministicNarrator { return &DeterministicNarrator{} }

func (DeterministicNarrator) Name() string { return "deterministic" }

func (DeterministicNarrator) Narrate(_ context.Context, b InsightBrief) (string, error) {
	var sb strings.Builder

	// Paragraph 1 — the headline metrics for the period.
	fmt.Fprintf(&sb, "Briefing for the %s. ", b.Period)
	if len(b.Headlines) > 0 {
		sb.WriteString(strings.Join(b.Headlines, " "))
	} else {
		sb.WriteString("No sales were recorded in this period.")
	}

	// Paragraph 2 — risks, worst first.
	crit := filterSeverity(b.Anomalies, "critical")
	warn := filterSeverity(b.Anomalies, "warn")
	info := filterSeverity(b.Anomalies, "info")
	if len(crit) > 0 || len(warn) > 0 {
		sb.WriteString("\n\n")
		if len(crit) > 0 {
			fmt.Fprintf(&sb, "Needs attention now: %s. ", joinDetails(crit))
		}
		if len(warn) > 0 {
			fmt.Fprintf(&sb, "Worth watching: %s. ", joinDetails(warn))
		}
	} else {
		sb.WriteString("\n\nNo risks were flagged this period.")
	}

	// Positive signals (acquisition, growth) as a short closing note.
	if len(info) > 0 {
		fmt.Fprintf(&sb, "On the upside: %s.", joinTitles(info))
	}

	// Paragraph 3 — the first concrete recommendation, if any.
	if rec := firstRecommendation(b.Anomalies); rec != "" {
		fmt.Fprintf(&sb, "\n\nRecommended next step: %s.", strings.TrimRight(rec, "."))
	}

	return strings.TrimSpace(sb.String()), nil
}

func filterSeverity(as []BriefAnomaly, sev string) []BriefAnomaly {
	var out []BriefAnomaly
	for _, a := range as {
		if a.Severity == sev {
			out = append(out, a)
		}
	}
	return out
}

// joinDetails renders "Title (detail)" clauses joined with semicolons.
func joinDetails(as []BriefAnomaly) string {
	parts := make([]string, 0, len(as))
	for _, a := range as {
		if a.Detail != "" {
			parts = append(parts, fmt.Sprintf("%s (%s)", a.Title, strings.TrimRight(a.Detail, ".")))
		} else {
			parts = append(parts, a.Title)
		}
	}
	return strings.Join(parts, "; ")
}

func joinTitles(as []BriefAnomaly) string {
	parts := make([]string, 0, len(as))
	for _, a := range as {
		if a.Title == "" {
			continue
		}
		parts = append(parts, strings.ToLower(a.Title[:1])+a.Title[1:])
	}
	return strings.Join(parts, ", ")
}

func firstRecommendation(as []BriefAnomaly) string {
	// Prefer the highest-severity recommendation.
	for _, sev := range []string{"critical", "warn", "info"} {
		for _, a := range as {
			if a.Severity == sev && a.Recommendation != "" {
				return a.Recommendation
			}
		}
	}
	return ""
}

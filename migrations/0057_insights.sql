-- 0057_insights.sql — Executive insights + AI-in-workflow digests.
-- The reporting module answers "what are my numbers" (live KPIs, custom report
-- builder). Insights answers "what changed, why it matters, and what to do" — a
-- periodic engine that computes period-over-period metrics, detects anomalies
-- with deterministic rules, and narrates an executive briefing (the narrative is
-- AI-authored when a provider is configured, deterministic-templated otherwise).
-- A weekly background job materialises one digest per active org and emails it;
-- the same engine also serves a live, on-demand metrics view for the dashboard.
--
-- This is "AI integrated into the daily workflow" rather than a chatbot: the
-- system produces the briefing proactively and it lands on the dashboard + inbox.

CREATE TABLE insight_digests (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),
  period_start    date        NOT NULL,
  period_end      date        NOT NULL,
  generated_at    timestamptz NOT NULL DEFAULT now(),
  -- Which engine wrote the narrative: 'deterministic' (offline template) or the
  -- provider name ('claude' | 'openai'). Lets the UI label AI-authored briefings.
  source          text        NOT NULL DEFAULT 'deterministic',
  trigger         text        NOT NULL DEFAULT 'schedule' CHECK (trigger IN ('schedule','manual')),
  narrative       text        NOT NULL DEFAULT '',
  kpis            JSONB       NOT NULL DEFAULT '{}'::jsonb,   -- {revenue, orders, aov, *_prev, *_delta_pct, ...}
  anomalies       JSONB       NOT NULL DEFAULT '[]'::jsonb    -- [{key,severity,title,detail,metric,recommendation,action}]
);
CREATE INDEX idx_insight_digests_org ON insight_digests (organization_id, generated_at DESC);

-- Insights reuse the existing reporting permissions (report.view to read,
-- report.manage to trigger a fresh digest), so no new role wiring is needed.

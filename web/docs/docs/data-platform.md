---
slug: /data-platform
title: Insights, audit & exports
sidebar_position: 8
---

# Insights, audit & exports

The data layer answers "what changed, why it matters, what to do" and "who did what" — three subsystems that share the live-aggregation, org-scoped, permission-gated conventions used everywhere else.

## Executive insights (`internal/insights`)

The insights engine computes period-over-period business metrics, detects anomalies with **deterministic, explainable rules**, and turns them into a written executive briefing. It is AI woven into the workflow, not a chatbot: a **weekly background job** (`run_insight_digests` → fan-out → `generate_insight_digest`) materialises one digest per active org and emails it; the same engine also serves a live, on-demand metrics view.

- **`Build(ctx, q, orgID, now, windowDays)`** → a `Snapshot`: revenue/orders/AOV with growth vs the prior window, gross margin, AR aging, churn-risk accounts, revenue concentration, new-logo acquisition, low stock. All live, date-windowed aggregates over `orders`/`order_items` (no materialization).
- **`Detect(snapshot)`** → ranked `[]Anomaly`, each with a severity, a plain-language explanation, a recommendation, and an `Action` deep-link into the relevant admin screen.
- **`ai.Narrator`** (mirrors `ai.PageDesigner`): `ClaudeNarrator` / `OpenAINarrator` author the briefing from the structured facts (told to invent no numbers); `DeterministicNarrator` is the always-on fallback. The engine falls back to deterministic if an AI call fails, so a digest is always produced.
- HTTP: `GET /admin/insights/metrics` (live), `/latest`, `/` (list), `/{id}`, `POST /generate`. Gated by `report.view` / `report.manage`.

**Margin** is "at current cost": `MarginWindow` joins `order_items` to the product's current `cost_price`. A per-line cost snapshot (for exact historical margin) is the next refinement.

## Audit trail (`internal/audit`, `internal/modules/audit`)

One middleware (`audit.Recorder.Middleware`) sits innermost in the authenticated chain and **automatically records every state-changing admin/vendor request** — actor, action (path-classified, e.g. `orders.confirm`), entity, method, path, status, IP, user-agent, request id. Coverage doesn't depend on instrumenting handlers.

- **Enrichment** is opt-in: a handler calls `audit.SetEntity / SetSummary / SetChange(before, after)` to attach a field-level diff (wired for customers + products).
- **Beyond the middleware**: `Recorder.Record(r, audit.Event{...})` records actions the middleware can't see — **login** attempts (`auth.login` / `auth.login_failed` / `auth.login_blocked`, on the public login route) and **exports** (`exports.download`).
- Writes are best-effort on a detached context (survive client disconnect). The `audit_log` table carries the FORCEd `org_isolation` RLS policy.
- HTTP: `GET /admin/audit` (filter + paginate), `/{id}`, `/export` (CSV). Gated by the sensitive `audit.view` permission.

## Data export center (`internal/export`, `internal/modules/exports`)

Full-record exports of orders, order line items, customers and invoices — distinct from the report builder (which aggregates).

- **`internal/export`** is a pure encoder layer: `WriteCSV`, a streaming `CSVStream`, and a minimal pure-Go `WriteXLSX` (stdlib `archive/zip` + hand-written XML — no third-party dependency; numeric columns become real numeric cells).
- **CSV streams from a database cursor** — unbounded with constant memory, so industrial-size datasets export fine. **XLSX** is built in memory and capped.
- Each dataset is a `dataset{}` in `buildDatasets()` (raw org-scoped SQL + a `scan` func → `[]string`); add one + its key to the `order` slice and the manifest + both formats are automatic.
- **Least privilege**: the manifest + center need `report.view`; each download is additionally gated by that entity's own view permission (e.g. `customer.view`). Exports are recorded in the audit trail.
- HTTP: `GET /admin/exports` (manifest of datasets the caller may export), `GET /admin/exports/{dataset}?format=csv|xlsx`.

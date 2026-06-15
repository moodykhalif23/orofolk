# Teggo Platform Roadmap — from commerce app to PIM/DXP platform

> **Thesis.** Data is the oil. The platform's job is to refine even the faintest
> signal into meaning, and to keep large industries in sync. Teggo already runs
> the whole commercial machine (commerce, pricing, CRM, order-to-cash,
> marketplace, insights); this roadmap closes the gaps that make it a credible
> **PIM/DXP platform**, not just a strong B2B commerce app.

This is a living document. It is the build plan behind the gap analysis of
Teggo's capabilities against a best-in-class PIM/DXP benchmark (Pimcore). It is
organised by **Tier 1** — the five gaps that, closed, make Teggo a credible
platform — sequenced into shippable phases.

---

## Where Teggo already leads (the moat)

Pimcore stops at "manage the data". Teggo runs the machine on top of it, and
this is deliberately **not** on the roadmap to change — it's the moat:

- Full commerce engine — cart, checkout, quotes, RFQ, orders, invoices, returns, order-to-cash.
- Sophisticated B2B pricing — price lists, volume tiers, customer/group/website hierarchy with ancestor fallback.
- CRM pipeline + opportunities, marketplace/multi-vendor (commission, payouts).
- Deep B2B integration rails — cXML punchout, EDI X12 (850/855/856/810), ERP sync.
- A real DAM — checksum dedup, transformation presets/renditions, signed delivery.
- A workflow/state-machine engine + approval routing.
- Commerce-grade, AI-narrated insights — margin, AR aging, churn, AOV.
- SaaS infrastructure — multi-tenant RLS, self-serve provisioning, billing/plans/metering/feature-gates.

---

## The sequencing insight

Two of the five Tier-1 items secretly contain **primitives** the others depend
on — **API keys** (inside Onboarding) and the **outbound event bus** (inside
Syndication). Pulled out and built first, they are small and they unblock the
rest. And the strategic headline — data quality — doesn't have to wait for the
big modeling work, because **completeness scoring is architected against the
attribute-*family* abstraction that already exists**, so it works on products
today and extends to custom objects for free when modeling lands.

```
Phase 0  API keys ──────────────┬──────────────► Onboarding (P3), Brand portal (P5)
         Outbound event bus ────┴──────────────► Syndication / Zapier-n8n (P4)

Phase 1  Data quality & health  ◄── scores against attribute families (exists today)
                │ (re-uses the same family abstraction)
Phase 2  Generic data modeling ─┬──► supercharges P1 (score any model)
                                └──► enables P3 (import any model), P4 (project any model)
```

---

## Phase 0 — Primitives (the unlock layer) · **Shipped**

Small, low-risk, high-leverage. Each reuses an existing pattern; nothing is greenfield.

| Deliverable | Builds on | Effort | Status |
|---|---|---|---|
| **API keys / scoped service tokens** — hashed keys (`tgk_…`, only the SHA-256 hash stored), scopes drawn from the existing permission catalog so `RequirePermission` gates key traffic unchanged, rotation, revoke, debounced last-used. Presented as `Authorization: Bearer tgk_…`. | `internal/auth`, the permission middleware | M | Shipped |
| **Outbound event bus + webhook delivery** — endpoints subscribe to domain events; the existing `EmitEvent → dispatch_event` worker fans each event out to matching endpoints as river jobs, so retries/backoff and a per-attempt delivery log come for free. Signed `X-Teggo-Signature` (HMAC-SHA256). Replay supported. | `internal/automation` dispatcher (events already fire), River workers, HMAC pattern from `internal/erp` | M | Shipped |

**Strategic payoff:** the moment webhooks exist, Zapier/n8n/Make integrate
themselves — "keep large industries in sync" starts here, because event
emission already exists internally.

**Implementation notes (as built):**
- API-key auth is added as a second path inside the request authenticator: a
  bearer value with the `tgk_` prefix is verified against `api_keys` (by hash)
  and resolves to synthesized admin claims carrying the key's org + scopes;
  anything else is parsed as a JWT. Zero change to existing call sites
  (`Authenticator(issuer)` delegates to `AuthenticatorWithKeys(issuer, nil)`).
- A key's scopes are validated at creation against the **creator's own**
  permissions, so a key can never exceed its minter's authority.
- Webhook delivery rides the existing per-event worker (`DispatchEventWorker`),
  which already runs in-app notifications + automation; webhook fan-out is added
  alongside, keyed by the org now carried on the event job.

**Shipped this iteration (backend):** migrations `0064_api_keys` + `0065_webhooks`;
sqlc queries + typed layer; `internal/apikey` + `internal/webhook` services (unit-tested);
the event fan-out + `deliver_webhook` river worker; admin modules
`internal/modules/apikeys` + `internal/modules/webhooks`; authenticator + server +
`cmd/api` wiring; OpenAPI paths + regenerated typed client. Verified: `sqlc generate`
deterministic, `go build`/`go vet` clean, unit tests pass, client typecheck clean.

**Shipped this iteration (admin UI):** `web/admin/src/views/developers/ApiKeysView.vue`
(list · create with scope picker drawn from the caller's own permissions · rotate ·
revoke · secret revealed exactly once) and `WebhooksView.vue` (list · create/edit ·
rotate signing secret · delete · delivery log with per-attempt status + replay).
Both routed, added to the **Settings** nav, and gated by `apikey.*` / `webhook.*`.
Verified: admin `vue-tsc --noEmit` clean against the regenerated client.

**Remaining for Phase 0:** a one-line backfill of the new permissions onto
existing (non-org-1) tenants (new tenants already inherit them via the
provisioning template).

---

## Phase 1 — Data quality & completeness *(the strategic headline)* · **Shipped**

"Is my catalog complete and correct?" — the purest expression of *visualise the
slightest data into meaning*.

| Deliverable | Builds on | Effort |
|---|---|---|
| **Completeness scoring engine** — score each entity against its attribute family's required/recommended fields → a 0–100 health score, live | attribute families, insights engine pattern | M |
| **Validation rules engine** — per-attribute constraints (regex, range, allowed-values, cross-field), applied on write *and* on import | attribute `data_type`/`options` already in schema | M |
| **Data-health dashboard** — completeness by category/family, worst-offenders, trend, "what's missing" enrichment queue | live dashboards + report builder | M |

Architected against families, so when Phase 2 generalises families to any
object, this scores everything automatically. Gate as a premium feature.

**Shipped this iteration (slice 1 — completeness scoring + dashboard):** the
scoring SQL (`internal/store/queries/dataquality.sql` — per-product score against
its family's *required* attributes, treating JSON `null`/`""`/`[]`/`{}` as
unfilled), the `GET /admin/data-health/catalog` endpoint
(`internal/modules/dataquality`, gated by a new `dataquality.view` permission —
migration `0067`, granted to existing tenants' admin/staff/viewer), OpenAPI +
regenerated client, and the **Catalog → Data health** admin screen (org score,
fully-complete vs incomplete counts, and a worst-offenders table showing each
product's missing required attributes — the enrichment work-list). Verified:
`sqlc` deterministic, `go build`/`vet` clean, a real-Postgres scoring test
(`internal/modules/dataquality/scoring_test.go`), and admin + client typechecks.

**Shipped this iteration (slice 2 — validation rules engine):** per-attribute
constraints stored on the attribute (migration `0068` adds `attributes.validation`),
a pure engine (`internal/validation`) enforcing regex + length (text), numeric
range (number/price), and selection-count + allowed-values (multiselect/select),
wired into **both** product writes (`adminCreate`/`adminUpdate` → `422` with the
violation list) and **CSV import** (per-row rejection). Attributes are now editable
(`PUT /admin/attributes/{id}`), and the admin **Catalog → Attributes** screen
authors rules + allowed options per type. Verified: engine unit tests, an
end-to-end real-Postgres test (invalid value → 422, valid → 201), the full catalog
suite still green, and client + admin typechecks clean.

**Remaining for Phase 1 (optional polish, neither blocks Phase 2):** a
**completeness trend** over time, and a **premium feature-gate** decision —
data-health + validation are permission-gated only today, so they're visible on
every plan.

---

## Phase 2 — Generic data modeling *(the platform unlock)* · **Shipped**

Model anything — assets, locations, contracts, suppliers — not just products.
This is what turns Teggo from "a commerce app with flexible product fields" into
a platform.

| Deliverable | Builds on | Effort |
|---|---|---|
| **Custom object types** — define entity types with field schemas; products become the first native object type conceptually | JSONB + attribute/family foundation proven on products | L |
| **No-code model/field builder UI** — define types, fields, families, validation without code | attribute admin UI patterns | M |
| **Generic CRUD + RLS + API** — every custom type gets org-scoped endpoints, RLS policy, audit, permissions automatically | RLS convention + isolation test gate, module pattern | L |

Retroactively supercharges Phase 1 and unblocks Phases 3–4.

**Shipped this iteration (slice 1 — custom-object backend):** three RLS-isolated
tables (migration `0069`: `object_types`, `object_fields`, `object_records`) and a
generic `objects` module with full CRUD for **types**, their **field schema**, and
**records** — records are validated against their type's fields by the **same
Phase-1 engine** (so `{"rating":6}` is rejected `422` against a `max:5` rule),
with audit capture and `object.view`/`object.manage` gating. A type with records
can't be dropped (`409` guard). Verified: real-Postgres flow test (define type →
field rule → record reject/accept → list → delete-guard → 404), the isolation RLS
gate over all three new tables, sqlc deterministic, client typecheck clean.

**Shipped this iteration (slice 2 — no-code builder UI):** **Settings → Custom
objects** — a types list (create/edit/delete) and a per-type detail with a
**field-schema builder** (the AttributesView rule editor, reused) plus a
**schema-driven record form** that renders one typed input per field and surfaces
the engine's `422` violations inline. Verified: admin `vue-tsc` clean.

**Shipped this iteration (slice 3 — completeness generalized):** the same scoring
rule now runs over each custom object type's required fields — new queries
(`ObjectTypeCompleteness`, `ObjectRecordCompletenessWorst`), endpoints
`GET /admin/data-health/objects` (per-type overview) and
`/admin/data-health/objects/{code}` (worst records + the fields each is missing),
and the **Data health** screen gained a **Custom objects** section with per-type
scores and a drill-in to incomplete records. Verified: real-Postgres test (type +
required field → complete/incomplete records → correct overview + worst), the full
dataquality + isolation suites, and client + admin typechecks.

**Phase 2 is complete** — completeness *and* validation now work uniformly across
products and every custom object type, all on one engine. Next: **Phase 3
(onboarding / ETL)**.

---

## Phase 3 — Onboarding at scale · **COMPLETE** (slices 1–4 shipped)

| Deliverable | Builds on | Effort |
|---|---|---|
| **Generic import engine** — any entity/model, CSV/Excel/JSON, column-mapping UI, dry-run + validation preview (uses Phase 1 rules), per-row results | generalise the product CSV importer | L |
| **Data matching & cleansing** — dedup on configurable keys, fuzzy match, normalisation on ingest | checksum-dedup precedent in DAM | M |
| **Supplier onboarding** — scoped API keys (Phase 0) + import templates so partners feed data in | Phase 0 + this engine | S |

**Shipped this iteration (slice 1 — generic import engine):** migration `0070`
adds `import_runs` + `import_rows`; a target abstraction with **two targets —
products *and* any custom object type — on one pipeline**; endpoints to list
targets, download a CSV template, **upload → dry-run** (parse + map + validate
every row, staged with per-row create/update/error verdicts, *nothing written*),
preview, and **commit** (apply create/update rows in one transaction). Every row
is checked by the Phase-1 validation engine; `import.view` / `import.manage` gate
it. Verified: real-Postgres test (products CSV create/update/error → commit, with
"nothing before commit" + re-commit-guard assertions; object records JSON with a
validation rejection), the isolation RLS gate over both tables, and client
typecheck. Object records are insert-only for now; the legacy product-import
endpoint stays until the UI moves to the engine.

**Shipped this iteration (slice 2 — import UI):** a reusable **Import data** screen
(Settings → Import data) — pick a target, download its CSV template, upload, see a
**dry-run preview** (create/update/error counts + per-row outcomes), then commit,
with a recent-imports list. Wired in: the product screen's Import button and a new
Import button on each custom object type route to it, so the UI runs on the engine
(the legacy inline product importer is retired). Verified: admin `vue-tsc`.

**Shipped this iteration (slice 3 — matching, cleansing & XLSX):** custom-object
records now **upsert** on a configurable **match field** (migration `0071` stores the
run's options; a `data->>field` lookup decides create vs update, re-applied on
commit); **cleansing** normalises string values on ingest (trim/lower/upper/collapse,
configurable); and a **pure-Go XLSX reader** (inline + shared strings) lets uploads be
`.xlsx` as well as CSV/JSON. The Import screen gained a match-field selector, clean-up
options, and `.xlsx` acceptance. Verified: real-Postgres tests (object
upsert-by-match; products + objects flow), an XLSX round-trip unit test, the
isolation gate, and client + admin typechecks.

**Shipped this iteration (slice 4 — supplier onboarding):** a **one-shot ingest
endpoint** (`POST /admin/imports/ingest`) validates AND applies rows in a single
call — no dry-run/commit round trip — so an external partner can feed data straight
in; the run is still recorded (status `committed`) and every row's outcome returned,
so rejects come back with a reason while valid rows land. It's scoped to a new
**`import.ingest`** permission (migration `0072`, granted to every org's admin so an
admin can mint a supplier key carrying it); a supplier key holds *only* that scope —
it can feed data and discover targets, nothing else (least privilege, proven in test).
Target discovery now carries a **machine-readable schema** per column (code, type,
required, allowed values, rule summary) — the "per-target template" a partner codes
against — reachable by both an interactive admin and an ingest-scoped key via a new
`RequireAnyPermission` gate. The Import screen gained a **partner-API contract panel**
(endpoint, field table, copy-able `curl`, link to mint a scoped key). The upload and
commit paths were refactored onto shared `prepare`/`planRows`/`stageRun`/`applyRun`
helpers that ingest reuses, so all three paths share one validated pipeline. Verified:
real-Postgres ingest **authenticated by an actual API key** (valid row applied + bad
row rejected in one call, run committed, least-privilege 403 on runs, discovery 200),
the `RequireAnyPermission` unit test, the existing upload/commit suite (no drift from
the refactor), the isolation gate, sqlc deterministic, and client + admin typechecks.

**Phase 3 is complete** — the generic engine takes products and any custom object
type, via CSV/XLSX/JSON, through the UI or a scoped partner key, all on one validated,
matched, cleansed pipeline. Next: **Phase 4 (syndication & distribution / feeds)**.

---

## Phase 4 — Syndication & distribution

| Deliverable | Builds on | Effort |
|---|---|---|
| **Feed builder** — project any model into a channel schema (Google Shopping, Amazon, custom JSON/XML/CSV), scheduled regeneration | Phase 2 models + report/export streaming | M |
| **Channel adapters** — pluggable per destination | adapter-registry pattern (AI/payment/blob already do this) | M |

Outbound webhooks shipped in Phase 0, so this phase is *feeds* specifically.

---

## Phase 5 — Brand / asset experience portal

| Deliverable | Builds on | Effort |
|---|---|---|
| **Asset lifecycle state** — draft/approved/published + expiry on assets | DAM | S |
| **External portal surface** — search/filter/share approved assets; partner access via Phase 0 API keys; share links with TTL | DAM signed delivery | M |

Self-contained; can slot earlier if a brand-partner deal demands it.

---

## At a glance

| Horizon | Phases | Theme |
|---|---|---|
| **Now** | 0 + 1 | Primitives + data-health visualisation (the headline) |
| **Next** | 2 | Generic modeling (platform unlock) |
| **Later** | 3 → 4 → 5 | Onboarding, syndication, portal |

## Cross-cutting definition of done

Every phase honours these conventions before it's "done":

- A migration with an **RLS `org_isolation` policy** (the isolation test gate enforces it on every org-scoped table).
- **OpenAPI** spec updates — the spec is the single source of truth for the typed client.
- New entries in the **permission catalog** + middleware gating.
- A **billing feature-gate** where the capability is premium.
- **Audit-trail** coverage for state-changing routes.
- An **admin UI** surface.

> New permissions are seeded onto the demo (org 1) admin role; the
> tenant-provisioning template copies org-1 admin permissions to new tenants, so
> new orgs inherit them automatically. Reaching pre-existing tenants is a
> one-line backfill migration when needed.

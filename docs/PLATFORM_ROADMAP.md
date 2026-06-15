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

## Phase 0 — Primitives (the unlock layer) · **Backend shipped · admin UI next**

Small, low-risk, high-leverage. Each reuses an existing pattern; nothing is greenfield.

| Deliverable | Builds on | Effort | Status |
|---|---|---|---|
| **API keys / scoped service tokens** — hashed keys (`tgk_…`, only the SHA-256 hash stored), scopes drawn from the existing permission catalog so `RequirePermission` gates key traffic unchanged, rotation, revoke, debounced last-used. Presented as `Authorization: Bearer tgk_…`. | `internal/auth`, the permission middleware | M | Backend shipped · UI pending |
| **Outbound event bus + webhook delivery** — endpoints subscribe to domain events; the existing `EmitEvent → dispatch_event` worker fans each event out to matching endpoints as river jobs, so retries/backoff and a per-attempt delivery log come for free. Signed `X-Teggo-Signature` (HMAC-SHA256). Replay supported. | `internal/automation` dispatcher (events already fire), River workers, HMAC pattern from `internal/erp` | M | Backend shipped · UI pending |

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
**Remaining for Phase 0:** the admin UI screens (manage keys; manage webhooks +
view deliveries) and a backfill of the new permissions onto existing (non-org-1) tenants.

---

## Phase 1 — Data quality & completeness *(the strategic headline)*

"Is my catalog complete and correct?" — the purest expression of *visualise the
slightest data into meaning*.

| Deliverable | Builds on | Effort |
|---|---|---|
| **Completeness scoring engine** — score each entity against its attribute family's required/recommended fields → a 0–100 health score, live | attribute families, insights engine pattern | M |
| **Validation rules engine** — per-attribute constraints (regex, range, allowed-values, cross-field), applied on write *and* on import | attribute `data_type`/`options` already in schema | M |
| **Data-health dashboard** — completeness by category/family, worst-offenders, trend, "what's missing" enrichment queue | live dashboards + report builder | M |

Architected against families, so when Phase 2 generalises families to any
object, this scores everything automatically. Gate as a premium feature.

---

## Phase 2 — Generic data modeling *(the platform unlock)*

Model anything — assets, locations, contracts, suppliers — not just products.
This is what turns Teggo from "a commerce app with flexible product fields" into
a platform.

| Deliverable | Builds on | Effort |
|---|---|---|
| **Custom object types** — define entity types with field schemas; products become the first native object type conceptually | JSONB + attribute/family foundation proven on products | L |
| **No-code model/field builder UI** — define types, fields, families, validation without code | attribute admin UI patterns | M |
| **Generic CRUD + RLS + API** — every custom type gets org-scoped endpoints, RLS policy, audit, permissions automatically | RLS convention + isolation test gate, module pattern | L |

Retroactively supercharges Phase 1 and unblocks Phases 3–4.

---

## Phase 3 — Onboarding at scale

| Deliverable | Builds on | Effort |
|---|---|---|
| **Generic import engine** — any entity/model, CSV/Excel/JSON, column-mapping UI, dry-run + validation preview (uses Phase 1 rules), per-row results | generalise the product CSV importer | L |
| **Data matching & cleansing** — dedup on configurable keys, fuzzy match, normalisation on ingest | checksum-dedup precedent in DAM | M |
| **Supplier onboarding** — scoped API keys (Phase 0) + import templates so partners feed data in | Phase 0 + this engine | S |

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

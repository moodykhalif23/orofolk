# Teggo —  B2B Commerce Platform

*The operating system for selling to other businesses.*

A self-hosted, **API-first** B2B commerce platform for manufacturers, distributors, and
wholesalers . It models organizations buying from
organizations: per-customer catalogs and pricing, quote negotiation (RFQ → Quote → Order),
order-to-cash (shipments, invoices, payments), an audited inventory ledger, and a marketplace
with third-party vendors.

Built as a **modular monolith**: one Go service is the single source of truth, and every
frontend is a pure API consumer. The OpenAPI contract is generated into a typed TypeScript
client, so the apps cannot drift from the API.

## Architecture

| Layer | Stack |
|---|---|
| **API** | **Go** — `chi` router · `sqlc` (type-safe SQL) · `river` (Postgres-backed job queue) |
| **Database** | **PostgreSQL 16** — also carries the job queue (river) and full-text search |
| **Admin SPA** | **Vue 3** (Vite · Pinia · Vue Router · **PrimeVue**) — login-gated back office · `:5173` |
| **Storefront** | **Nuxt** SSR (**PrimeVue**) — crawlable, customer-facing · `:3000` |
| **Vendor portal** | **Vue 3** (Vite · **PrimeVue**) — marketplace sellers: products, orders, payouts · `:5174` |
| **API client** | **OpenAPI 3.1** → generated **TypeScript** types (`openapi-typescript` + `openapi-fetch`) |
| **PDF / Edge** | Gotenberg (invoice PDFs) · Docker Compose |

The three frontends share one PrimeVue **Aura** base with a custom **Teggo** preset
(indigo primary + amber accent).
``

## Prerequisites

- **Go** ≥ 1.25 · **Docker** (for Compose, and for integration tests via testcontainers)
- **Node** ≥ 20 + **pnpm 9** (`corepack enable pnpm`) — for the frontends
- **sqlc** (only if you change SQL): `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`

---

## Run the backend (Docker Compose)

```bash
cp .env.example .env          # then set JWT_SECRET (openssl rand -base64 32)
docker compose up --build
```

Compose starts Postgres, runs migrations + demo seed (one-shot `migrate` service), then the
API and worker. The API is on **http://localhost:8080**.

### Verify it's up

```bash
curl http://localhost:8080/healthz                      # {"status":"ok"}
curl 'http://localhost:8080/storefront/products'        # seeded products (public)

# Admin login → bearer token (seeded demo admin)
curl -s -X POST http://localhost:8080/admin/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@demo.test","password":"admin1234","org_id":1}'
```

### Demo logins (development seed)

| App | Email | Password | Notes |
|---|---|---|---|
| Admin SPA | `admin@demo.test` | `admin1234` | org `1` |
| Storefront | `buyer@demo.test` | `buyer1234` | Demo Buyer Co |
| Vendor portal | `vendor@demo.test` | `vendor1234` | Demo Vendor Co |

The seed hashes are real bcrypt of those passwords. **Change or remove the seed migrations
(`migrations/0003_seed.sql`, `0040_seed_demo_buyer.sql`, `0042_seed_demo_vendor.sql`) before
production.**

---

## Run the frontends

### One command (backend + GUIs)

```bash
corepack enable pnpm     # once — installs the pnpm shim
make dev                 # backend (Docker, detached) + all three dev servers
```

`make dev` runs [`scripts/dev.sh`](scripts/dev.sh): it brings the backend up with
`docker compose up -d`, installs web deps, then starts the frontends. Ctrl-C stops the
frontends; the backend stays up (`make down` to stop it).

| Make target | What it does | URL |
|---|---|---|
| `make dev` | Backend (detached) + admin + storefront + vendor | — |
| `make web` | admin + storefront + vendor dev servers (backend assumed up) | — |
| `make admin` | admin SPA only | http://localhost:5173 |
| `make storefront` | storefront only | http://localhost:3000 |
| `make vendor` | vendor portal only | http://localhost:5174 |
| `make docs` | generate API reference + serve docs | http://localhost:3001 |
| `make api-client` | regenerate the typed client from the OpenAPI spec | — |
| `make web-build` | production build of admin + storefront + vendor + docs | — |

### Or with pnpm directly

```bash
cd web
corepack enable pnpm
pnpm install
pnpm --filter @teggo/api generate     # typed client from packages/api/openapi.yaml

pnpm --filter @teggo/admin dev        # admin SPA  → http://localhost:5173
pnpm --filter @teggo/storefront dev   # storefront → http://localhost:3000
pnpm --filter @teggo/vendor dev       # vendor     → http://localhost:5174
pnpm -r build                         # production build of all packages
```

The admin Vite dev server proxies `/admin` and `/storefront` to the API at `localhost:8080`
(override with `VITE_API_BASE_URL`). The storefront reads `NUXT_PUBLIC_API_BASE`
(default `http://localhost:8080`). See [web/README.md](web/README.md).

---

## Tests

```bash
make test          # go test ./... — integration tests start Postgres via testcontainers (needs Docker)
make vet           # go vet ./...
make fmt           # gofmt -w .
```

Set `TEST_DATABASE_URL` to run integration tests against an existing Postgres instead of a
throwaway container (e.g. in CI). Frontend checks:

```bash
cd web
pnpm --filter @teggo/admin typecheck
pnpm --filter @teggo/storefront typecheck
pnpm --filter @teggo/vendor typecheck
```

---

## Code generation

| Generator | When | Command |
|---|---|---|
| **sqlc** — typed Go from `internal/store/queries/*.sql` into `internal/store/gen` | after editing SQL | `make generate` |
| **TypeScript client** — from `web/packages/api/openapi.yaml` | after editing the OpenAPI spec | `make api-client` |

The OpenAPI file is the **single source of truth** for the API contract; all frontends consume
the generated types, so they cannot drift.

---

## Adding a backend module (the repeatable pattern)

1. Add a `migrations/00NN_<name>.sql`.
2. Add `internal/store/queries/<name>.sql`, then `make generate`.
3. Create `internal/modules/<name>/handler.go` with a `Routes(r chi.Router, authMW ...)` method —
   org-scoped, permission/audience gated.
4. Mount it in `internal/server/server.go`.
5. Async work → add a job in `internal/queue/jobs/` and register it in `internal/queue/client.go`.
6. Add integration tests (real Postgres via `testsupport.NewDB(t)`): query-level + HTTP-level
   (assert the auth gate and tenant isolation).
7. Extend `web/packages/api/openapi.yaml`, run `make api-client`, and build the screens.

Conventions — money as decimal strings, `public_id` in URLs, `organization_id` tenant scoping,
`text + CHECK` statuses, JSONB + GIN attributes — are documented in [MANUAL.md](MANUAL.md).

---

## Documentation

| Doc | Covers |
|---|---|
| [MANUAL.md](MANUAL.md) | Product & operations manual — what each module does and how it fits together |
| [web/README.md](web/README.md) | Frontend workspace guide |
| `make docs` | Developer docs + interactive API reference (Docusaurus) → http://localhost:3001 |

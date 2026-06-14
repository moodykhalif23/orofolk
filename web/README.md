# Teggo frontends

Three apps in one pnpm workspace, all consuming the Go API (the single source of truth) through
a shared, generated TypeScript client.

| App | Path | Stack | Role |
|---|---|---|---|
| **Admin SPA** | [`admin/`](admin/) | Vue 3 · Vite · Pinia · Vue Router · **PrimeVue** | Login-gated back office (data grids, CRUD, dashboards). No SEO. `:5173` |
| **Storefront** | [`storefront/`](storefront/) | Nuxt · SSR · **PrimeVue** | Customer-facing, crawlable storefront (SEO-critical). `:3000` |
| **Vendor portal** | [`vendor/`](vendor/) | Vue 3 · Vite · **PrimeVue** | Marketplace sellers: products, orders, payouts. `:5174` |
| **API client** | [`packages/api/`](packages/api/) | `openapi-typescript` + `openapi-fetch` | Typed client generated from `openapi.yaml`. |
| **Docs** | [`docs/`](docs/) | Docusaurus | Developer docs + interactive API reference. |

## Quick start

```bash
corepack enable pnpm        # installs the pnpm shim if missing
cd web
pnpm install
pnpm --filter @teggo/api generate     # regenerate the typed client from packages/api/openapi.yaml

pnpm --filter @teggo/admin dev        # Admin SPA      → http://localhost:5173
pnpm --filter @teggo/storefront dev   # Storefront     → http://localhost:3000
pnpm --filter @teggo/vendor dev       # Vendor portal  → http://localhost:5174
```

Each app reads the API base URL from an env var (`VITE_API_BASE_URL` for admin and vendor,
`NUXT_PUBLIC_API_BASE` for the storefront) — default `http://localhost:8080` (the Go API).
Set it in an app-level `.env` (e.g. `web/admin/.env`) to override.

## Conventions

- **No business logic in the frontend.** Every app is a pure API consumer; the Go service owns
  all rules. The UI is presentation + orchestration only.
- **OpenAPI is the source of truth** for the API contract. The typed client in
  [`packages/api`](packages/api/) is generated from `openapi.yaml` (`pnpm --filter @teggo/api
  generate`); all three apps import `@teggo/api`, so they cannot drift from the API.
- **PrimeVue v4** on the **Aura** base with a custom **Teggo** preset (indigo primary + amber
  accent, via `@primeuix/themes`). The admin and vendor SPAs import components per-SFC; the
  Nuxt storefront auto-imports them via `@primevue/nuxt-module`.
- **Distinct security contexts** mirror the backend: the admin SPA uses a bearer JWT (`/admin/*`),
  the storefront uses a customer-user session (`/storefront/*`), and the vendor portal uses a
  vendor-user session (`/vendor/*`).

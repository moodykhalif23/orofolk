# Teggo Developer Docs

Docusaurus site for the Teggo platform: handwritten developer **guides** plus an
**API reference** generated from the OpenAPI contract at
[`../packages/api/openapi.yaml`](../packages/api/openapi.yaml), so the docs never
drift from the generated TypeScript client.

## Run

```bash
pnpm install                       # from web/ (workspace root)
pnpm --filter @teggo/docs start    # copies spec, generates API docs, serves on :3000
```

> Run pnpm scripts via `--filter` from `web/`. If a `pnpm --filter` pre-run deps
> check ever fails on `ERR_PNPM_IGNORED_BUILDS`, invoke the binary directly:
> `cd web/docs && node ./node_modules/@docusaurus/core/bin/docusaurus.mjs <cmd>`.

## Build

```bash
pnpm --filter @teggo/docs build    # → web/docs/build (static site)
```

`build` runs `copy-spec` → `clean-api` → `gen-api` → `docusaurus build`.

## Layout

- `docs/*.md` — handwritten guides (intro, getting-started, architecture,
  conventions, module-pattern, auth-rbac, data-layer, background-jobs,
  integrations, frontend, configuration). Listed in `sidebars.ts` under `guides`.
- `docs/api/` — **generated** (git-ignored). Run `pnpm gen-api` to (re)create.
- `static/openapi.yaml` — **generated** copy of the spec for the download button.
- `docusaurus.config.ts` — docs-only site; OpenAPI plugin → `../packages/api`.

## Version pins (don't bump casually)

The `@docusaurus/*` stack and `webpack` are pinned in
[`../pnpm-workspace.yaml`](../pnpm-workspace.yaml) `overrides`:
- Docusaurus `3.8.1` — matches `docusaurus-theme-openapi-docs` Tabs internals
  (newer core breaks SSG with `useTabsContext() must be used within a Tabs component`).
- webpack `5.97.1` — newer releases reject webpackbar's `ProgressPlugin` options.

The `webpackTweaks` plugin in `docusaurus.config.ts` also (a) stubs the Node core
modules `postman-code-generators` pulls in, and (b) `ignoreWarnings`-filters one benign
`export 'default' ... SchemaTabs ... was not found` notice — the theme ships SchemaTabs
as CJS, so the client compile's static analysis flags a default that resolves fine at
runtime. If you ever hit a *real* SchemaTabs error, narrow or drop that filter first.

After editing an endpoint in the OpenAPI spec, just rebuild — the API reference
regenerates from it.

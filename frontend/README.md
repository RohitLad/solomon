# Solomon frontend

Svelte 5 + Vite + Tailwind (shadcn-style tokens) single-page app: Compose /
Queue / Analytics / Evergreen / Accounts tabs. Talks to the Go backend on
`:8080` — in dev, `/api` and `/uploads` are proxied there (see `vite.config.ts`),
so no CORS or env config is needed.

Full guides: [`../docs/USER_MANUAL.md`](../docs/USER_MANUAL.md) (end users) ·
[`../docs/DEV_MANUAL.md`](../docs/DEV_MANUAL.md) (architecture, API, recipes).

## Commands

```bash
npm run dev          # dev server on :5173 (needs the backend running — see make dev-bg)
npm run build        # production build → dist/
npm run check        # svelte-check + tsc (same as CI gate in make vet)
npm run test         # vitest run — unit suites (normalize guards, …)
npm run test:watch   # vitest watch mode for development
```

From the repo root, `make dev-bg` / `make stop` / `make test` / `make vet`
cover both services at once — prefer those.

## Structure

```
src/
  App.svelte            # all five tabs + all server-data wiring
  main.ts / app.css     # mount + Tailwind + CSS-var token layer
  lib/api.ts            # typed API client (api.*) + NETWORKS pill meta
  lib/normalize.ts      # asArray/asRecord null-guards (+ normalize.test.ts)
  lib/components/ui/    # Button, Card, Input, Textarea, Badge
```

## Rule: never consume a list payload raw

Go nil slices serialize as `null`, so **every list-typed API field must go
through `asArray()`** (records: `asRecord()`) **before `.map()` / `.length` /
`{#each}`** — in script code and (via `?? []`) in markup. This once
white-screened the app (`can't access property "map", exp is null`); the
backend guarantees `[]` too, but the UI must not trust it. If you wire a new
endpoint, wrap it and extend `normalize.test.ts`.

## Recommended IDE setup

[VS Code](https://code.visualstudio.com/) +
[Svelte](https://marketplace.visualstudio.com/items?itemName=svelte.svelte-vscode).

# Solomon — Developer Manual

Architecture, code map, data model, API reference, auth flows, and step-by-step
recipes for extending the app. Companion to `USER_MANUAL.md` (end-user guide)
and `README.md` (quick start).

---

## Table of contents

1. [Architecture at a glance](#1-architecture-at-a-glance)
2. [Repository layout (every file)](#2-repository-layout-every-file)
3. [Backend deep dive](#3-backend-deep-dive)
4. [Data model (SQLite schema)](#4-data-model-sqlite-schema)
5. [REST API reference (every endpoint)](#5-rest-api-reference-every-endpoint)
6. [Networks layer (limits, publish, OAuth, AI, stats, tokens, smart)](#6-networks-layer)
7. [Scheduler workers](#7-scheduler-workers)
8. [Frontend deep dive](#8-frontend-deep-dive)
9. [Configuration (every env var)](#9-configuration-every-env-var)
10. [OAuth & token flows per network](#10-oauth--token-flows-per-network)
11. [Recipes (add/change/extend)](#11-recipes-addchangeextend)
12. [Testing cookbook (curl e2e)](#12-testing-cookbook-curl-e2e)
13. [Build, run & deploy](#13-build-run--deploy)
14. [Conventions, gotchas & troubleshooting](#14-conventions-gotchas--troubleshooting)

---

## 1. Architecture at a glance

```
┌─────────────┐  /api/*   ┌──────────────────────────────┐
│ Svelte SPA  │◄─────────►│ Go Fiber API (:8080)         │
│ :5173 (dev) │  (proxy)  │ handlers → networks → SQLite │
└─────────────┘           │ scheduler tickers (15s/60s/1h)│
                          └──────────────────────────────┘
```

- **Backend:** Go, Fiber v2 (HTTP), GORM + `mattn/go-sqlite3` (SQLite, auto-migrated),
  `godotenv` (.env), `google/uuid` (IDs). OAuth/token HTTP calls are raw
  `net/http` — no OAuth library (see `networks/oauth.go`, `networks/tokens.go`).
- **Frontend:** Svelte 5 + Vite, Tailwind v3 with shadcn-style CSS tokens, hand-rolled
  `ui/` primitives (Button/Card/Input/Textarea/Badge) mirroring shadcn-svelte,
  one `App.svelte` with six tabs, typed `lib/api.ts` client.
- **Concurrency model:** `PublishDuePost` fans out to targets with a goroutine per
  target + `sync.WaitGroup`, then rolls up post status
  (`published` / `partial` / `failed`).
- **Demo principle:** any account whose token is empty or starts with `demo`
  takes the mock path in every network function — the whole product is exercisable
  with zero credentials.

## 2. Repository layout (every file)

```
solomon/
├── Makefile                 # ★ every command: install/env/dev(-bg)/dev-backend/dev-frontend/
│                           #   build/vet/test/health/smoke(/-live)/stop/clean
├── README.md                    # quick start + feature map + live-keys guide
├── docs/
│   ├── USER_MANUAL.md           # this file's companion (end users)
│   └── DEV_MANUAL.md            # you are here
├── backend/
│   ├── main.go                  # app wiring: middleware, static, ALL routes, scheduler start
│   ├── go.mod / go.sum          # modules (fiber, gorm, sqlite driver, uuid, godotenv)
│   ├── .env.example             # every env var with comments (copy to .env)
│   ├── solomon.db               # created at runtime (gitignore in real use)
│   ├── uploads/                 # created at runtime; served at /uploads/*
│   ├── db/db.go                 # Connect(path): open SQLite + AutoMigrate 6 models
│   ├── models/                  # one file per model (see doc.go for the map):
│   │   │                            # network / social_account / media / post (+targets) /
│   │   │                            # analytics / external / evergreen (+ BeforeCreate UUIDs)
│   ├── networks/
│   │   ├── limits.go            # ★ limits table + ValidateAndAdapt + SplitThread + TrimTo
│   │   ├── publisher.go         # Publish dispatch + 7 network impls + extraField parser
│   │   ├── discover.go          # ListRecentPosts per network (Outside Solomon, best-effort)
│   │   ├── oauth.go             # AuthURL consent links per network
│   │   ├── ai.go                # GenerateCaptions: offline templates → optional LLM
│   │   ├── smart.go             # DefaultSlots best-time tables per network
│   │   ├── stats.go             # FetchStats: demo pseudo-stats + live fetch attempts
│   │   └── tokens.go            # RefreshToken per-network OAuth refresh grants
│   ├── handlers/
│   │   ├── accounts.go          # List/Create/Delete/AuthURL/Limits
│   │   ├── posts.go             # List/Create/PATCH-reschedule/Preview/Delete (split semantics §3.2)
│   │   ├── discover.go          # DiscoverExternal worker (native posts, Buffer-parity caps)
│   │   ├── upload.go            # multipart → MediaAsset rows (+ UploadDir, /uploads static)
│   │   ├── publish.go           # PublishDuePost: validate→skip-or-publish fan-out + rollup
│   │   ├── ai.go                # POST /api/ai/captions
│   │   ├── schedule.go          # GET /api/schedule/suggest + engagementBoost + BestSlot
│   │   ├── analytics.go         # GET /api/analytics + POST /api/analytics/refresh + RefreshAll
│   │   ├── tokens.go            # GET /api/accounts/expiring + POST /api/accounts/refresh + RefreshDueAccounts
│   │   ├── bulk.go              # POST /api/posts/bulk (CSV) + evergreen CRUD + RunDueRules
│   │   ├── lists_test.go        # ★ empty-DB contract: every list returns [], never null
│   │   ├── calendar_test.go     # ★ PATCH/delete/evergreen/deleted-guard contract tests
│   │   └── discover_test.go     # ★ outside-discovery contract tests (dedupe/caps/freeze/union)
│   └── scheduler/scheduler.go   # Start(): 4 goroutines — 15s posts, 60s evergreen, 1h tokens+discovery
└── frontend/
    ├── package.json / vite.config.ts   # vite + svelte plugin; dev proxy /api,/uploads → :8080
    ├── tailwind.config.js / postcss.config.js
    ├── index.html
    └── src/
        ├── main.ts / app.css    # mount + Tailwind + shadcn token layer
        ├── App.svelte           # 6 tabs: Compose / Queue / Calendar / Analytics / Evergreen / Accounts
        ├── lib/api.ts           # types (Account/Post/…) + api.* client (22 methods) + NETWORKS meta
        ├── lib/normalize.ts     # ★ asArray/asRecord null-guards for every list payload
        │                        #   (+ normalize.test.ts, run with `npm run test` / vitest)
        ├── lib/calendar.ts      # ★ pure calendar helpers: monthGrid/groupByDay/dropDateTime/dotClass
        │                        #   (+ calendar.test.ts) — App.svelte stays thin
        ├── lib/Counter.svelte   # vite scaffold leftover (unused, safe to delete)
        └── lib/components/ui/   # Button, Card, Input, Textarea, Badge (shadcn-style)
```

## 3. Backend deep dive

### 3.1 `main.go` — wiring

- `fiber.New(BodyLimit: 100MB)` → `logger` + open `cors` → `app.Static("/uploads", UploadDir())`.
- Route groups under `/api` (full table in §5). Handlers are structs holding `*gorm.DB`
  (`AccountHandler`, `PostHandler`, `TokenHandler`, `AnalyticsHandler`,
  `ScheduleHandler`, `BulkHandler`, `EvergreenHandler`); `Captions`/`Limits`/upload are plain funcs.
- `scheduler.Start(db, 15s)` then `Listen(":"+PORT)`.

### 3.2 Request flows

- **Create post:** `POST /api/posts` → validate → insert `Post` → attach media IDs
  (`media_ids` → set `post_id`, `sort_order`) → insert `PostTarget`s →
  if `publish_now`: synchronous `PublishDuePost`; elif `scheduled_at`: `scheduled`;
  elif `auto_schedule`: `BestSlot(db, targets)` → `scheduled`; else `draft`.
- **Publish path (`handlers/publish.go`):** load post+media → per target goroutine:
  load account → `EffectiveText` (CustomText wins) → `ValidateAndAdapt` →
  `!OK` ⇒ `skipped` + joined errors; else `networks.Publish(...)` ⇒
  `published` (+ comma-joined IDs, note in `error` col) or `failed` (+ err).
  Rollup: all published → `published`; none + some failed → `failed`; else `partial`.
  Note: targets already `published` are skipped on re-run (idempotent-ish retries).
- **Preview (`POST /api/posts/preview`):** same validation, no writes; returns per-target
  `{account_id, network, text, plan{ok, adaptations[], errors[]}}` + full limits map.
- **Reschedule (`PATCH /api/posts/:id {scheduled_at}`):** draft/scheduled only
  (404 unknown, 400 published/partial/failed/deleted — duplicate instead);
  null clears to draft, datetime sets scheduled. Past datetimes allowed (the
  15s scheduler publishes them at once; UI confirms).
- **Delete (split semantics):** draft/scheduled → HARD delete (target/media
  rows, unreferenced media files, evergreen-pool prune). Published/partial/
  failed → SOFT delete (`deleted_at` set; targets/snapshots/media KEPT for
  analytics history with a `deleted` badge). Network copies are NEVER touched
  (industry standard — Buffer/Hootsuite don't un-publish either).
- **Upload:** multipart field `files`; extension allowlist
  (images: jpg/jpeg/png/gif/webp; video: mp4/mov/webm/mkv); stored
  `<unixnano>_<8hex><ext>`; row created with empty `post_id` until attached.
- **Bulk:** CSV → per row resolve accounts (name- or network-match, `;`-separated) →
  optional `auto_schedule` spacing (+1h start, +3h steps) → create post+targets.
  Date parsing: RFC3339, else `2006-01-02 15:04`.
- **Evergreen run:** due rule → `pool[cursor%len]` → deep-copy post (new `Post`,
  new `MediaAsset` rows pointing at same `FilePath`, new targets) →
  `PublishDuePost` → `cursor++`, `last_run_at=now`, `next_run_at=now+interval`.

### 3.3 Key helpers

- `networks.extraField(extra, key)` — parses Extra as JSON `map[string]string`,
  else `k=v,k=v` pairs.
- `Target.EffectiveText(master)` — CustomText override.
- `TrimTo(s, max)` — rune-safe trim + `…`.
- `SplitThread(text, max)` — greedy word-wrap into `max-10` chunks, hard-cuts
  oversize words, suffixes `(i/n)`.

## 4. Data model (SQLite schema)

Auto-migrated by `db.Connect`. All IDs are UUID strings.

**social_accounts:** `id PK, network (index), name, external_id, access_token ⚠️,
refresh_token ⚠️, expires_at, extra, is_active (default true), created_at`.
Tokens are stripped (`""`) in every JSON response — grep for `AccessToken, … = "", ""`
before adding new serializers.

**posts:** `id PK, title, content, link, scheduled_at NULL, status (draft/scheduled/
published/partial/failed), deleted_at NULL (index; soft-delete for published history),
post_type (reserved), created_at, updated_at`. List hides `deleted_at IS NOT NULL`
unless `?include_deleted=1`.

**post_targets:** `id PK, post_id (index), account_id (index), custom_text,
first_comment, status (pending/published/failed/skipped), network_post_id
(comma-joined for threads), error (doubles as note holder on success), published_at`.

**media_assets:** `id PK, post_id (index), file_path (local path), media_type
(image/video), sort_order`.

**analytics_snapshots:** `id PK, target_id (index), external_post_id NULL (index),
network_post_id, views, likes, comments, shares, is_demo, fetched_at`. Append-only;
"latest" = `ORDER BY fetched_at DESC LIMIT 1`. Exactly one of target/external set.

**external_posts:** `id PK, account_id (index), network, network_post_id`
(unique per account), `text, permalink, published_at (index), last_stats_at, created_at`.
Native posts discovered per account (read-only: analytics + calendar dots).

**evergreen_rules:** `id PK, name, pool_post_ids (JSON []postID), account_ids
(JSON []accountID), interval_hours, cursor, active, last_run_at, next_run_at, created_at`.

## 5. REST API reference (every endpoint)

Base `http://localhost:8080`. Errors are `{"error": "…"}` with 4xx/5xx.

**Empty-collection contract:** every list-typed field is ALWAYS a JSON array —
`[]` when empty, never `null` (Go nil slices are initialized before encoding;
pinned by `handlers/lists_test.go`). The frontend additionally normalizes every
list payload through `lib/normalize.ts` (`asArray`/`asRecord`) before
`.map()`/`.length`/`{#each}` — keep both sides; either alone prevents a
white-screen.

| Method & path | Body / query | Returns |
|---|---|---|
| `GET /api/health` | — | `{"ok":true}` |
| `GET /api/limits` | — | `{network: Limit{max_chars,max_images,max_videos,max_video_seconds,max_video_mb,text_only_allowed,needs_title,notes}}` |
| `GET /api/accounts` | — | `Account[]` (tokens blanked), ordered by network,name |
| `POST /api/accounts` | `{network*, name*, external_id, access_token, refresh_token, extra}` — empty token ⇒ `demo-<network>`, expiry +60d | `201 Account` |
| `DELETE /api/accounts/:id` | — | `{"ok":true}` |
| `GET /api/accounts/:network/auth-url` | — | `{"auth_url"}` (see §10) |
| `GET /api/accounts/expiring` | — | accounts with `expires_at` NULL or ≤ now+7d |
| `POST /api/accounts/refresh` | — | `{"refreshed":n, "errors":[]}` |
| `GET /api/posts?status=` | optional status filter (+ `?include_deleted=1` reveals soft-deleted) | `Post[]` (targets+account, media), newest first |
| `POST /api/posts` | `{title, content*, link, scheduled_at, publish_now, auto_schedule, media_ids[], targets[]:{account_id*, custom_text, first_comment}}` | `201 Post` (with targets; publish_now runs synchronously) |
| `PATCH /api/posts/:id` | `{scheduled_at}` (null clears to draft) — draft/scheduled only | updated `Post` (404 unknown, 400 published/deleted) |
| `POST /api/posts/preview` | `{title, content, link, media_types[], targets[]}` | `{targets[]:{account_id,network,text,plan}, limits}` |
| `DELETE /api/posts/:id` | — | draft/scheduled: hard delete; published: soft delete `{ok, soft_deleted}` |
| `POST /api/upload` | multipart `files` | `201 MediaAsset[]` |
| `POST /api/posts/bulk[?auto_schedule=1]` | multipart `file` (.csv) | `{created, skipped, errors[]}` |
| `POST /api/ai/captions` | `{text*, network}` | `{variants[]:{tone,text,hashtags}, source, limits}` |
| `GET /api/schedule/suggest?networks=&count=` | csv networks (default all), count 1–20 (default 3) | `{suggestions[]:{at, score, networks, reason}}` |
| `GET /api/analytics` | — | `{totals, outside:{views,likes,comments,shares,posts}, rows[]}` (rows carry `deleted`/`external` flags; external rows add `text`/`published_at`) |
| `POST /api/analytics/refresh` | — | `{"refreshed":n, "discovered":m, "errors":[]}` (also runs Outside discovery) |
| `GET /api/evergreen` | — | `EvergreenRule[]` newest first |
| `POST /api/evergreen` | `{name*, pool_post_ids[]*, account_ids[]*, interval_hours}` (default 24) | `201 Rule` (`next_run_at = now+interval`) |
| `DELETE /api/evergreen/:id` | — | `{"ok":true}` |

## 6. Networks layer

One concern per file; **start here when an API changes upstream.**

- **`limits.go`** — `Limits()` table (the only place network caps live),
  `ValidateAndAdapt(network, text, title, nImages, nVideos, hasLink) → PlanResult{ok, adaptations[], errors[]}`.
  Policy recap: X over-limit ⇒ `split_thread`; LinkedIn/Pinterest over-limit ⇒ `trim`;
  FB/IG/YT/TikTok over-limit ⇒ `trim` note; IG/TikTok/Pinterest/YT without media ⇒ hard error
  (caller converts to `skipped`); YT needs video+title; Pinterest missing title/link ⇒ advisory adaptations.
- **`publisher.go`** — `Publish(PublishInput{Account, Title, Text, Link, FirstComment, MediaPaths, IsVideo}) → PublishResult{NetworkPostIDs, Note}`.
  Demo short-circuit per network (200ms sleep + `mockIDs`). Real paths use `httpClient`
  (60s timeout); media-upload-heavy flows (X chunked, YT resumable, TikTok FILE_UPLOAD,
  LinkedIn asset registration, Pinterest multipart) are marked `NOTE:` with exact endpoints —
  local-file→public-URL promotion via `PUBLIC_MEDIA_BASE_URL` is the documented prod path.
- **`oauth.go`** — `AuthURL(network)` builds consent URLs from env client IDs +
  `OAUTH_REDIRECT_BASE + /<network>`; scopes listed per provider (2025–26).
- **`ai.go`** — `GenerateCaptions(text, network)`; `hashtagBank` per network
  (8 tags IG/TikTok, 3 others); X variants pre-trimmed to 240. LLM hook:
  `AI_API_KEY` + `AI_BASE_URL` (default OpenAI chat-completions) + `AI_MODEL`
  (default `gpt-4o-mini`), expects JSON array, tolerant fence-stripping; **any failure
  falls back to offline templates** (never 500s the UI).
- **`discover.go`** — `ListRecentPosts(network, token, extra, externalID, since, limit)`
  for Outside-Solomon discovery. Demo/empty tokens → deterministic `fnv`
  pseudo-posts. Real paths: FB page posts, IG media, X user timeline (needs
  numeric user ID in External ID), YT uploads playlist (+1 batched titles call),
  LinkedIn ugcPosts (needs `author_urn`), Pinterest board pins (needs `board_id`).
  TikTok returns "needs audited app" (caller skips with a note). Missing IDs or
  scopes are errors, never panics — the worker treats them as per-account notes.
- **`smart.go`** — `DefaultSlots(network) → []Slot{weekday(0=Sun), hour, score 1–3}`.
- **`stats.go`** — `FetchStats(network, token, postID) → Stats`. Demo = `fnv` hash
  pseudo-stats (stable per post ID). Live: IG insights parsed (`reach/likes/comments/shares/saved`);
  FB/Pinterest scaffolds with permission hints; YT/TikTok/LinkedIn/X return actionable errors
  (required scopes/endpoints named).
- **`tokens.go`** — `RefreshToken(network, access, refresh, extra) → RefreshResult`.
  Demo = extend 60d. Real: X `.../2/oauth2/token`, FB/IG `fb_exchange_token` (60d),
  Google `oauth2.googleapis.com/token`, TikTok `.../v2/oauth/token/`, LinkedIn
  `.../oauth/v2/accessToken` (partner-app caveat), Pinterest `.../v5/oauth/token`.

## 7. Scheduler workers

`Start(db, interval)` spawns four goroutines (no graceful shutdown — fine for v1):

1. **Due posts** every `interval` (15s from main): `status=scheduled AND scheduled_at<=now AND deleted_at IS NULL` → `PublishDuePost`.
2. **Evergreen** every 60s: `handlers.RunDueRules(db)` (active + `next_run_at<=now`; pool filtered to live posts, cursor advances past missing/deleted).
3. **Tokens + discovery** every 1h: `handlers.RefreshDueAccounts(db)` (expiry NULL or ≤ now+7d), then `handlers.DiscoverExternal(db)` (native posts: 30d backfill, ≤100/account, stats frozen after 20d, app IDs deduped).

## 8. Frontend deep dive

- **`lib/api.ts`** — `BASE=''`, `req<T>` (throws body text on !ok), types
  (`Network`, `Account` incl. `expires_at?`, `MediaAsset`, `PostTarget`, `Post`),
  `api.*` for all 21 methods, `NETWORKS[{id,label,color}]` pill meta.
- **`lib/normalize.ts`** — `asArray()`/`asRecord()`: every list-typed API field
  goes through these before `.map()`/`.length`/`{#each}` (see §5 contract).
  Covered by `lib/normalize.test.ts` (`npm run test`, vitest; `npm run test:watch`
  for watch mode). Rule: if you consume a new list endpoint in UI code, wrap it.
- **`App.svelte`** — state: `tab`, `accounts/posts/limits`, compose bundle
  (`title/content/link/scheduledAt/selected/customText/firstComment/showCustom/mediaIds/mediaTypes/preview`),
  AI (`variants/variantSource`), smart (`suggestions`), accounts (`newNet/newName/newToken/newExtra/expiringIds`),
  analytics (`totals/outside/arows`), evergreen (`rules/ruleName/ruleHours/rulePool/ruleAccts/bulkResult`),
  calendar (`calYear/calMonth/calDay/dragPost/confirmDelete/calMove`), notices (`notice`).
  Derived: `selectedIds`, `selectedNets`, `charCount` (rune-spread `[...content]`),
  `calGrid/calByDay/calExtByDay/calDrafts/calDayPosts/calDayExt` (pure helpers in
  `lib/calendar.ts` — keep date logic there, testable without Svelte).
  `switchTab` lazy-loads analytics/evergreen/expiring (+ analytics for calendar dots).
  `datetime-local` ↔ ISO conversion pads local components. `onBulk(e, auto)` posts CSV `FormData`.
  Delete flow is two-step (`askDelete` → inline confirm → `doDelete`); past-date
  moves use a native `confirm()`; duplicate loads text/targets into compose only
  (media re-attached by hand — same rows would be *moved*, and networks flag
  identical reposts as spam).
- **`ui/` primitives** — prop-driven (`variant/tone`), Tailwind classes on CSS-var
  tokens from `app.css` (`--background/--primary/…` + `.dark`); add new primitives here.
- **Vite proxy** (`vite.config.ts`): `/api` + `/uploads` → `localhost:8080`, so the
  SPA works unmodified in dev and behind any same-origin prod static host.

## 9. Configuration (every env var)

Loaded from `backend/.env` (copy `.env.example`) + process env. All optional unless noted.

| Var | Default | Used by / purpose |
|---|---|---|
| `PORT` | `8080` | `main.go` listen |
| `DB_PATH` | `./solomon.db` | `db.Connect` SQLite file |
| `UPLOAD_DIR` | `./uploads` | `UploadDir()` + `/uploads` static |
| `OAUTH_REDIRECT_BASE` | `http://localhost:8080/api/accounts/callback` | `oauth.go` redirect URIs |
| `PUBLIC_MEDIA_BASE_URL` | `http://localhost:8080/uploads/` | prod public media prefix (IG/FB/TikTok/YT) |
| `TWITTER_CLIENT_ID/_SECRET` | — | X OAuth + refresh |
| `FB_APP_ID/_SECRET` | — | FB/IG OAuth + `fb_exchange_token` |
| `GOOGLE_CLIENT_ID/_SECRET` | — | YouTube OAuth + refresh |
| `TIKTOK_CLIENT_KEY/_SECRET` | — | TikTok OAuth + refresh |
| `LINKEDIN_CLIENT_ID/_SECRET` | — | LinkedIn OAuth + refresh |
| `PINTEREST_APP_ID/_SECRET` | — | Pinterest OAuth + refresh |
| `AI_API_KEY` | — (offline) | `ai.go` LLM upgrade switch |
| `AI_BASE_URL` | OpenAI chat-completions | LLM endpoint (any OpenAI-compatible) |
| `AI_MODEL` | `gpt-4o-mini` | LLM model name |

## 10. OAuth & token flows per network

Callback exchange is intentionally left as "paste the token" (flows differ per
provider and rotate yearly) — the consent URL is generated in-app, exchange via
provider docs or the curl sketches below. Redirect URI must match the registered
one: `$OAUTH_REDIRECT_BASE/<network>`.

- **X:** `POST https://api.twitter.com/2/oauth2/token` (`grant_type=authorization_code`,
  `code`, `redirect_uri`, `code_verifier`, basic-auth client id/secret) → ask
  `tweet.read tweet.write users.read offline.access`.
- **FB/IG:** `GET graph.facebook.com/v21.0/oauth/access_token` (client_id/secret,
  `redirect_uri`, `code`) → Page token via `/{page-id}?fields=access_token`;
  long-lived via `fb_exchange_token`; resolve `ig_user_id` via `/{page-id}?fields=instagram_business_account`.
- **Google/YT:** `POST oauth2.googleapis.com/token` (`code` → `access_token` + `refresh_token`
  with `access_type=offline&prompt=consent`); upload with `youtube/v3` lib + token source.
- **TikTok:** `POST open.tiktokapis.com/v2/oauth/token/` (`client_key`, `code`) →
  `video.upload/video.publish`; refresh grant documented in `tokens.go`.
- **LinkedIn:** `POST linkedin.com/oauth/v2/accessToken` (`code` → 60d token);
  author URNs `urn:li:person:{id}` / `urn:li:organization:{id}`; header `LinkedIn-Version: 202401`.
- **Pinterest:** `POST api.pinterest.com/v5/oauth/token` (`code` → token + 365d refresh);
  `GET /v5/boards` for `board_id`.

## 11. Recipes (add/change/extend)

**A. Network limits changed upstream** → edit ONLY `networks/limits.go`
(`Limits()` row + `ValidateAndAdapt` branch). Frontend picks it up via `/api/limits`
(char counters, cheat-sheet, preview) with zero UI changes.

**B. Add a new network (e.g. Bluesky)** — checklist:
1. `models.go`: `NetworkBluesky Network = "bluesky"` + append in `AllNetworks()`.
2. `limits.go`: Limits row + `ValidateAndAdapt` media/text policy.
3. `publisher.go`: `publishBluesky(in)` + `case` in `Publish` (demo-first, then real).
4. `oauth.go`: `AuthURL` case; `tokens.go`: `RefreshToken` case; `stats.go`: `FetchStats` case.
5. `smart.go`: `DefaultSlots` case (else falls to X defaults — fine temporarily).
6. `frontend/src/lib/api.ts`: extend `Network` union + `NETWORKS` entry.
7. Docs: USER §10/§12 rows, README network list. Rebuild both, run §12 cookbook.

**C. Add an endpoint** — handler func/struct in `handlers/` (tokens blanked on accounts),
register in `main.go` api group, add `api.*` client + UI call. Keep handlers thin;
network logic belongs in `networks/`.

**D. Add a scheduler job** — new goroutine+ticker in `scheduler.Start` calling an
exported `handlers.Xxx(db)` (keeps scheduling policy testable without HTTP).

**E. New post type (polls, YT thumbnails, IG stories)** — extend `MediaAsset` or
`Post` fields via GORM (auto-migrates), extend relevant `publish*` + preview
branch + compose UI input.

**F. Swap SQLite → Postgres** — replace `db.Connect` driver (`gorm.io/driver/postgres`
+ `DATABASE_URL`), keep models untouched; note `AutoMigrate` stays for dev only —
add versioned migrations (e.g. `golang-migrate`) before multi-instance deploy.

## 12. Testing cookbook (curl e2e)

Three layers, fastest first:

1. **Unit suites — `make test`.** Backend: `go test ./...`
   (`handlers/lists_test.go` pins the empty-DB `[]`-not-`null` contract against a
   throwaway SQLite file, no server needed). Frontend: `npm run test` (vitest,
   `normalize.test.ts` pins the null-payload guards). Run before every commit.
2. **Mock e2e — `make smoke`.** Serves a throwaway DB on `:18081`, runs the full
   flow below (account → captions → suggest → threaded publish → analytics →
   bulk → evergreen → token refresh → draft reschedule (PATCH) → hard delete →
   soft delete → history+outside analytics assertions), prints `SMOKE PASSED`. Steps are
   `&&`-chained with a trap teardown: the first failure aborts (no false
   passes) and the temp server/DB is always cleaned up.
3. **Live-fire — `make smoke-live`.** The twin against real tokens (see below):
   same fail-fast chaining, but publishes clearly-labeled posts ONLY to
   sandbox-flagged accounts after typed confirmation.
   It copies your real DB to `/tmp`, serves it on `:18082`, discovers accounts
`/tmp`, serves it on `:18082`, discovers accounts flagged as sandboxes
(`smoke=true` — also accepts `test=true`/`sandbox=true` — in the account
Extra field), prints the exact targets, and only publishes clearly-labeled
`Solomon live smoke test <timestamp> — safe to delete` posts after you type
`LIVE`. Safety rules baked in: real DB never written (copy discarded after),
no token refresh (rotation could stale your real DB — read-only expiry check
instead), no bulk/evergreen (they schedule future live publishes), and the
printed network post IDs are yours to delete manually on each network.
The raw sequence (useful when debugging one step — always against a THROWAWAY
DB, never your real `solomon.db`) is:

```bash
cd backend && DB_PATH=/tmp/dev_cookbook.db PORT=8080 go run . & sleep 5
curl -s localhost:8080/api/health                                        # {"ok":true}
# accounts
curl -s -X POST localhost:8080/api/accounts -H 'Content-Type: application/json' \
  -d '{"network":"twitter","name":"@acme"}'
TW=$(curl -s localhost:8080/api/accounts | python3 -c "import json,sys;d=json.load(sys.stdin);print([a['id'] for a in d if a['network']=='twitter'][0])")
# captions + suggest
curl -s -X POST localhost:8080/api/ai/captions -H 'Content-Type: application/json' \
  -d '{"text":"We launched X","network":"linkedin"}'
curl -s "localhost:8080/api/schedule/suggest?networks=twitter,linkedin&count=2"
# thread remedy: 800+ chars → 3-tweet thread, media-less IG skipped, post partial
LONG=$(python3 -c "print('lorem ipsum dolor sit amet '*30)")
curl -s -X POST localhost:8080/api/posts -H 'Content-Type: application/json' \
  -d "{\"content\":\"$LONG\",\"publish_now\":true,\"targets\":[{\"account_id\":\"$TW\"}]}"
# analytics / tokens / bulk / evergreen
curl -s -X POST localhost:8080/api/analytics/refresh
curl -s localhost:8080/api/analytics | head -c 300; echo
curl -s localhost:8080/api/accounts/expiring; echo
curl -s -X POST localhost:8080/api/accounts/refresh; echo
printf 'title,content,link,scheduled_at,accounts\n"T1","Bulk one","https://example.com","","twitter"\n' > /tmp/b.csv
curl -s -X POST localhost:8080/api/posts/bulk -F "file=@/tmp/b.csv"; echo
P=$(curl -s 'localhost:8080/api/posts' | python3 -c "import json,sys;print(json.load(sys.stdin)[0]['id'])")
curl -s -X POST localhost:8080/api/evergreen -H 'Content-Type: application/json' \
  -d "{\"name\":\"tips\",\"pool_post_ids\":[\"$P\"],\"account_ids\":[\"$TW\"],\"interval_hours\":24}"
```

Expected: `201`s, `MOCK thread x3`, `refreshed≥1`, `created:1`, rule with future `next_run_at`.
Force-fire evergreen: `UPDATE evergreen_rules SET next_run_at='2020-01-01'` via sqlite,
wait 70s, `cursor` increments and a new published post appears.

## 13. Build, run & deploy

```bash
make install              # once: go mod tidy + npm install
make env                  # copy backend/.env.example → backend/.env (never overwrites)
make dev                  # split-terminal mode (prints one command per pane)
make dev-bg / make stop   # background mode (logs in .logs/, PIDs in .make.pids.*,
                          #   dev-bg waits for API health and fails fast if it never comes up)
make dev-backend / make dev-frontend   # foreground single-service runs
make build                # backend/solomon binary + frontend/dist/
make vet                  # go vet + svelte-check
make test                 # unit suites: go test ./... + vitest
make health               # curl /api/health (exits nonzero when the API is down)
make smoke                # throwaway-DB e2e on :18081 (fail-fast, always cleans up)
make smoke-live           # live-fire on sandbox-flagged accounts only (typed LIVE confirm)
make clean                # artifacts/logs (keeps solomon.db + uploads/)
```

Raw equivalents: `cd backend && go run .` (:8080), `cd frontend && npm run dev`
(:5173), `go build -o solomon .`, `npm run build` (→ `dist/`).

- **Single-host deploy:** systemd unit for `./solomon` (`PORT`, `DB_PATH` absolute),
  Caddy/nginx reverse-proxy with `/` → frontend `dist/` and `/api`, `/uploads` → `:8080`.
- **Backups:** `solomon.db` (SQLite — copy while quiet, or `.backup`) + `uploads/`.
- **Env:** real `.env` never committed; `PUBLIC_MEDIA_BASE_URL` must be public HTTPS in prod.
- **Scaling notes:** SQLite + in-process tickers = single instance. For multi-instance:
  Postgres (§11F), distributed lock around `PublishDuePost`, shared object storage for media.

## 14. Conventions, gotchas & troubleshooting

- **Keep it flat & obvious:** one file per concern, no DI framework, no ORM magic
  beyond AutoMigrate; a new contributor should trace any request in <5 min from `main.go`.
- **Tokens never leak:** blank `AccessToken/RefreshToken` in ALL account serializers
  (List, Create response, nested `Targets.Account`, expiring). Audit with
  `grep -rn AccessToken backend/handlers`.
- **cgo:** `mattn/go-sqlite3` needs gcc on build hosts (`modernc.org/sqlite` is the
  cgo-free alternative if cross-compiling).
- **`error` column dual-use:** success notes (e.g. `MOCK thread x3`) also land in
  `PostTarget.error` — UI slices it; don't treat non-empty as failure (check `status`).
- **Threads:** stats use the FIRST tweet ID (`network_post_id` before first comma).
- **Timezone:** suggestions/schedules use server-local time; `datetime-local` has no zone —
  keep server TZ stable (e.g. `TZ=Europe/Berlin`) or store UTC.
- **Empty means `[]`, never `null`:** Go nil slices encode as `null`, which once
  white-screened the UI (`can't access property "map", exp is null`). Both sides
  guard it — handlers initialize every list before `c.JSON` (see `lists_test.go`),
  and UI code wraps every list payload in `asArray()`/`asRecord()` from
  `lib/normalize.ts` (see `normalize.test.ts`). New list endpoint? Do both.
- **No un-publish, by design:** schedulers (Buffer/Hootsuite included) don't
  delete published network posts. Solomon follows: published deletes are
  in-app history (`deleted_at` + badge), network copies are removed natively.
  Don't add a `networks.Delete` without re-reading this thread.
- **Makefile chains are fail-fast:** `smoke`/`smoke-live` join steps with `&&`
  plus a trap teardown — a failed step aborts the run (and, for smoke-live,
  aborts BEFORE any live publish) while still killing the temp server and
  deleting temp files. Keep new steps in the same chain; don't reintroduce `;`
  separators between fallible steps.
- **Port busy:** `make stop` (sweeps `:8080`/`:5173`, covering orphaned `go run`/vite children).
  If make itself reports `missing separator`, your Makefile copy has spaces where
  tabs belong — re-fetch it; recipe lines MUST start with a tab.
- **Frontend a11y warnings** (labels/self-closing) are non-blocking; `Textarea` must stay
  `<textarea>...</textarea>` (Svelte rejects self-closing non-void elements).
- **Vite scaffold leftovers:** `Counter.svelte`, `assets/*` unused — safe to delete.
- **When stuck:** `make test`, `make vet`, `make health`, server log lines (`.logs/` in dev-bg mode, else stdout `scheduler: …`),
  then USER manual §13 table.

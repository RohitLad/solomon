# Solomon — social media scheduler

Go **Fiber** + **GORM/SQLite** backend, **Svelte SPA** (Tailwind + shadcn-svelte-style UI) frontend.
7 networks, latest official APIs (2025–26): **X/Twitter v2 · Facebook Graph v21 · Instagram Graph ·
YouTube Data v3 · TikTok Content Posting v2 · LinkedIn Posts API · Pinterest v5**.

> 📖 Manuals: **[User Manual](docs/USER_MANUAL.md)** (install → connect → publish → automate,
> every tab, every network, FAQ) · **[Developer Manual](docs/DEV_MANUAL.md)**
> (architecture, schema, API reference, auth flows, extension recipes).

## Quick start (demo mode — no API keys needed)

```bash
make install              # Go modules + npm packages (once)
make dev-bg               # start API (:8080) + web (:5173) in background…
                          # …or: make dev   # split-terminal mode (one command per pane)
make stop                 # stop background services
```

Raw commands (no make): `cd backend && go run .` + `cd frontend && npm run dev`.
Full target list: `make help`. Unit suites: `make test` (Go + vitest). Mock e2e:
`make smoke`. Live-fire test on sandbox accounts: `make smoke-live` (publishes
labeled posts ONLY to accounts with `smoke=true` in Extra, after typed confirmation).

1. Open the app → **Accounts** → add e.g. `twitter / @acme` (empty token = `demo` mock publish).
2. **Compose** → write text, attach images/video → tick several accounts → **Check limits** → **Post now** / **Schedule**.
3. **Queue** shows per-account status (`published / failed / skipped` + network post id or error).

## Project layout (kept deliberately flat & simple)

```
Makefile                # ★ every command: make help
                        # (install/env/dev/dev-bg/dev-backend/dev-frontend/build/vet/test/health/smoke/smoke-live/stop/clean)
backend/
  main.go                 # Fiber app + routes (all routes visible in one place)
  models/models.go        # SocialAccount, Post, PostTarget (per-account CustomText), MediaAsset,
                          #   AnalyticsSnapshot, EvergreenRule
  db/db.go                # SQLite + AutoMigrate
  networks/
    limits.go             # ★ ALL network limits + remedies (edit only this file when APIs change)
    publisher.go          # one publish func per network (mock when token = demo-*)
    oauth.go              # OAuth consent-URL per network
  handlers/
    accounts.go posts.go upload.go publish.go ai.go schedule.go analytics.go tokens.go bulk.go
    discover.go avatar.go  # Outside-Solomon discovery + profile-pic fetch workers
    lists_test.go         # ★ empty-DB contract: every list returns [], never null
    calendar_test.go      # ★ PATCH/delete/evergreen/deleted-guard contract tests
    discover_test.go      # ★ outside-discovery contract tests (dedupe/caps/freeze/union)
    avatar_test.go        # ★ avatar serialization (account + analytics rows)
  scheduler/scheduler.go  # 15s posts + 60s evergreen + 1h tokens/discovery tickers
frontend/
  src/lib/api.ts          # typed API client (22 methods)
  src/lib/normalize.ts    # ★ asArray/asRecord: null-safe guards for every list payload (+ tests)
  src/lib/social.ts       # ★ brand glyphs + avatar helpers (+ tests)
  src/lib/calendar.ts     # ★ pure calendar helpers: monthGrid/groupByDay/dropDateTime (+ tests)
  src/lib/components/ui/  # Button/Card/Input/Textarea/Badge/NetBadge/SocialIcon/Avatar
  src/App.svelte          # Compose / Queue / Calendar / Analytics / Evergreen / Accounts tabs
```

## How the requested features map

| Requirement | Where |
|---|---|
| shorts / post / pic / multi-pic / video | `MediaAsset` list per post; `post_type` derived; YT Shorts = ≤60s vertical + `#Shorts` in title/desc |
| multiple accounts × multiple networks | `SocialAccount` rows, any count per network |
| post to many accounts at once | `Post` + N `PostTarget`s, published concurrently (`publish.go`) |
| per-account tweaks of one post | `PostTarget.CustomText` + `FirstComment` (UI "Customize" toggle) |
| network limits + remedies | `networks/limits.go` `ValidateAndAdapt` + `/api/posts/preview` ("Check limits") |
| X over 280 chars | `SplitThread` → numbered reply-chain thread |
| IG/YT/TikTok/Pinterest w/o media | target **skipped** w/ explanation instead of failing the whole post |
| LinkedIn/Pinterest over-limit | trim + overflow to first comment; Pinterest auto-title from first 100 chars |

## Going live (per network)

Copy `backend/.env.example` → `backend/.env`, fill client IDs/secrets, then per account open
`GET /api/accounts/:network/auth-url`, complete consent, exchange `code` for tokens
(scopes are listed in `networks/oauth.go`), and `POST /api/accounts` with the real token.
Network specifics:

- **X**: OAuth 2.0 PKCE; media needs chunked upload (`upload.twitter.com`), then `media_ids` on first tweet of thread.
- **Facebook**: page token needs `pages_manage_posts`; multi-photo = unpublished `/photos` + `attached_media`; Reels = `/video_reels`.
- **Instagram**: Business/Creator account linked to a FB Page; media must be a **public URL** → set `PUBLIC_MEDIA_BASE_URL`; carousels = children `is_carousel_item=true`.
- **YouTube**: `google.golang.org/api/youtube/v3` resumable `videos.insert`; Shorts ≤60s vertical + `#Shorts`.
- **TikTok**: `FILE_UPLOAD` init → chunk PUT → publish; honor `privacy_level/duet/stitch` flags; refresh via `/v2/oauth/token/`.
- **LinkedIn**: register image/video upload → PUT binary → `POST /v2/posts` with `LinkedIn-Version: 202401`; `author_urn` in account Extra.
- **Pinterest**: `POST /v5/pins` with `board_id` (in Extra), title ≤100, description ≤800, destination link.

## API cheatsheet

```
GET  /api/health  /api/limits
GET  POST /api/accounts   DELETE /api/accounts/:id   GET /api/accounts/:network/auth-url
GET  /api/accounts/expiring   POST /api/accounts/refresh   (token auto-refresh, hourly worker)
GET  POST /api/posts      POST /api/posts/preview    DELETE /api/posts/:id
POST /api/upload  (multipart field "files")
POST /api/posts/bulk?auto_schedule=1  (multipart "file", CSV: title,content,link,scheduled_at,accounts)
POST /api/ai/captions  {text, network}  (offline templates; LLM via AI_API_KEY/AI_BASE_URL/AI_MODEL)
GET  /api/schedule/suggest?networks=twitter,linkedin&count=3  (analytics-boosted best times)
GET  /api/analytics   POST /api/analytics/refresh
GET  POST /api/evergreen   DELETE /api/evergreen/:id  (recycled every interval_hours, 60s worker)
```

## New in v2 — the five features

1. **Smart scheduling** — `networks/smart.go` holds best-time defaults per network;
   `GET /api/schedule/suggest` adds +2 for YOUR top weekday-hours by engagement.
   Compose → ⚡ Best time fills the slot; **Auto-schedule** picks it on submit.
2. **AI captions** — Compose → ✨ Variants: 3 tones + per-network hashtag bank.
   Set `AI_API_KEY` (+ optional `AI_BASE_URL`/`AI_MODEL`) for real LLM output, else offline templates.
3. **Analytics** — Analytics tab: totals + per-target views/likes/comments/shares.
   `POST /api/analytics/refresh` re-pulls (demo = deterministic pseudo-stats;
   Instagram insights live; other networks' exact scopes/endpoints in `networks/stats.go`).
4. **Token auto-refresh** — hourly worker refreshes tokens expiring within 7d via each
   network's OAuth refresh grant (`networks/tokens.go`); Accounts tab shows
   expires/expired badges + manual ↻ Refresh tokens.
5. **Bulk & evergreen** — Evergreen tab: CSV import (accounts matched by name or
   network, `;`-separated) with optional 3h-spaced auto-schedule; rules recycle a pool
   of posts round-robin to chosen accounts every N hours (scheduler runs due rules).

## New in v3 — calendar, honest delete, Outside Solomon

6. **Calendar** — month grid with per-day status dots, drag-to-reschedule
   (keeps time of day; past drops confirm since due posts publish in ~15s),
   unscheduled-drafts tray, click-empty-day to compose, duplicate-with-edit-nudge.
   `PATCH /api/posts/:id` powers moves (draft/scheduled only).
7. **Honest delete** — scheduled/drafts delete forever; published posts are
   removed from Solomon but kept in Analytics with a `deleted` badge (network
   copies are never touched — industry standard).
8. **Outside Solomon** — native posts made outside the app are discovered per
   account (manual refresh + hourly; 30-day backfill, stats frozen after 20d),
   shown in their own Analytics section and as dimmed calendar dots, and feed
   the Best-time boost with your real history.

## New in v4 — brand identity, avatars, professional polish

9. **Social logos + avatars** — brand-color glyphs on every network pill and
   badge; real profile photos (fetched at connect/refresh, initials fallback
   with network logo badge) everywhere accounts appear.
10. **Polish** — product mark + segmented tab bar, status-accented queue cards,
    guided empty states, focus rings, dead-scaffold cleanup.

## Suggested next features (tell me which to build)

1. Best-time-to-post heatmap + auto-schedule queue
2. First-comment automation (hashtag banks per network)
3. Twitter polls, YT thumbnails/end-screens, IG story support
4. Link-in-bio page + UTM builder, link click tracking
5. Token auto-refresh workers + expiry alerts
6. Team workspaces, approvals, Coffee/Calendar drag-drop
7. Analytics pull (views/likes per network post id)
8. AI caption variants + hashtag suggestions per network
9. Bulk CSV import + recurring/evergreen repost rules
10. Webhooks (post published/failed → Slack/Discord)

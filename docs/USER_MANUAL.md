# Solomon — User Manual

Everything you need to install, connect, publish, schedule, measure, and automate
with Solomon, the 7-network social media scheduler
(X · Facebook · Instagram · YouTube · TikTok · LinkedIn · Pinterest).

---

## Table of contents

1. [What Solomon does](#1-what-solomon-does)
2. [Requirements](#2-requirements)
3. [Installation & first run](#3-installation--first-run)
4. [Demo mode vs live mode](#4-demo-mode-vs-live-mode)
5. [Accounts tab — connecting profiles](#5-accounts-tab--connecting-profiles)
6. [Compose tab — writing & publishing posts](#6-compose-tab--writing--publishing-posts)
7. [Queue tab — tracking every post](#7-queue-tab--tracking-every-post)
8. [Analytics tab — measuring results](#8-analytics-tab--measuring-results)
9. [Evergreen tab — bulk import & auto-recycling](#9-evergreen-tab--bulk-import--auto-recycling)
10. [Network reference (limits, requirements, remedies)](#10-network-reference-limits-requirements-remedies)
11. [Background automation (what runs while you sleep)](#11-background-automation-what-runs-while-you-sleep)
12. [Going live per network (OAuth, step by step)](#12-going-live-per-network-oauth-step-by-step)
13. [Troubleshooting & FAQ](#13-troubleshooting--faq)
14. [Glossary](#14-glossary)

---

## 1. What Solomon does

- Lets you connect **unlimited accounts** across **7 networks**.
- Lets you write **one master post** and publish it to **many accounts on many
  networks at the same time**, with optional **per-account tweaks**.
- Handles **text, single image, multi-image carousels, video, and Shorts/reels**.
- **Schedules** posts for later, suggests the **best time to post**, and can
  **auto-schedule** for you.
- Automatically fixes network mismatches: long text becomes an **X thread**,
  over-limit text is **trimmed with overflow in the first comment**, accounts
  that can't accept a post (e.g. Instagram without a photo) are **skipped with
  an explanation** instead of failing the whole post.
- Generates **AI caption variants + hashtags**, tracks **analytics
  (views/likes/comments/shares)**, **refreshes expiring tokens**, imports
  **bulk CSVs**, and **recycles evergreen posts forever**.

## 2. Requirements

| Tool | Version | Check with |
|------|---------|------------|
| Go | 1.23+ | `go version` |
| Node.js + npm | 18+ | `node --version` |
| GNU make (command shortcuts) | any (preinstalled on macOS) | `make --version` |
| A browser | any modern | — |
| (Live mode only) API credentials | per network | §12 |

No database server needed — Solomon uses an embedded **SQLite** file.

## 3. Installation & first run

You never need to memorize commands — the root `Makefile` has them all
(`make` or `make help` lists everything).

```bash
make install              # once: Go modules + npm packages
make dev-bg               # background mode: API (:8080) + web (:5173), logs in .logs/
make health               # sanity check → {"ok":true}
make stop                 # stop the background services
```

Prefer two terminal panes? `make dev` prints one command per pane and runs
nothing — run each line yourself. Without make, the raw equivalents are
`cd backend && go run .` and `cd frontend && npm run dev`.

First launch creates `backend/solomon.db` + `backend/uploads/` automatically.

Open **http://localhost:5173** in your browser. The frontend talks to the
backend automatically (dev proxy forwards `/api` and `/uploads` to port 8080).

> Data lives in `backend/solomon.db` (posts, accounts, analytics, rules) and
> `backend/uploads/` (your images/videos). Back up both files to back up everything.

## 4. Demo mode vs live mode

- **Demo mode (default):** when you add an account, leave the token empty and
  Solomon stores a `demo-<network>` token. Publishing is **simulated** —
  targets become `published` with IDs like `demo_twitter_…`, analytics show
  realistic sample numbers. Perfect for learning and testing.
- **Live mode:** paste a real OAuth access token per account (§12). Publishing
  hits the real network APIs.

Demo and live accounts can coexist — e.g. live X + demo TikTok.

## 5. Accounts tab — connecting profiles

### 5.1 Adding an account

1. Go to the **Accounts** tab.
2. Click the network pill (X, Facebook, Instagram, YouTube, TikTok, LinkedIn, Pinterest).
3. **Name** — anything recognizable: `@acme`, `Acme FB Page`, `Acme YT channel`.
4. **Access token** — leave empty for demo mode, or paste a real token (§12).
5. **Extra** — network-specific IDs (details below). Two formats accepted:
   - `key=value` pairs separated by commas: `page_id=123,ig_user_id=456`
   - JSON: `{"page_id":"123","ig_user_id":"456"}`
6. Click **Save account**.

Add as many accounts per network as you like — e.g. three X profiles plus two
Facebook Pages. Each account shows its **profile photo** (fetched when you
connect; demo accounts and unreachable photos show an initials circle) with a
tiny **network logo badge**, plus the network pill — so you can tell profiles
apart at a glance everywhere: Accounts, Compose, Evergreen, Analytics, and the
calendar.

### 5.2 Extra-field cheat sheet

| Network | Extra keys | Example |
|---------|-----------|---------|
| X/Twitter | (none needed) | — |
| Facebook | `page_id` (or put Page ID in External ID) | `page_id=10987654321` |
| Instagram | `ig_user_id` (+ linked FB Page) | `ig_user_id=178414…` |
| YouTube | (none; token is channel-bound) | — |
| TikTok | (none; token is user-bound) | — |
| LinkedIn | `author_urn` e.g. `urn:li:person:abc` or `urn:li:organization:123` | `author_urn=urn:li:organization:123` |
| Pinterest | `board_id` (**required** to publish) | `board_id=987654321` |

### 5.3 "Get OAuth URL" button

Opens that network's official login/consent page in a popup. After approving,
the provider redirects with a `code` — exchange it for tokens per §12, then
paste the access token into the account form. (Client IDs come from
`backend/.env` — copy `backend/.env.example` to `backend/.env` first.)

### 5.4 Expiry badges & token refresh

Next to each account you may see:

| Badge | Meaning | What to do |
|-------|---------|------------|
| *(none)* | Token healthy (>7 days left) | nothing |
| `expires <date>` (amber) | Expires within 7 days | click **↻ Refresh tokens**, or wait — the server auto-refreshes hourly |
| `expired` (red) | Already expired | click **↻ Refresh tokens**; if it fails, reconnect via OAuth |
| `no expiry` (amber) | No expiry recorded | harmless; refresh once to set one |

> Demo tokens never really expire — refreshing just extends them 60 days.

### 5.5 Flagging a sandbox account for live tests

`make smoke-live` publishes real (labeled, deletable) test posts — but ONLY to
accounts whose **Extra** field contains `smoke=true` (e.g. `smoke=true`, or
`board_id=123,smoke=true`). Create a dedicated sandbox account (private test
profile/page), add the flag, and never flag a production-audience account.

### 5.6 Removing an account

Click **remove**. Scheduled (not-yet-published) targets for that account stay in
history but will fail/skip at publish time; published history is kept.

## 6. Compose tab — writing & publishing posts

### 6.1 The fields

| Field | Required? | Notes |
|-------|-----------|-------|
| **Title** | Yes for YouTube & Pinterest (≤100 chars); optional elsewhere | YouTube = video title; Pinterest = pin title (auto-taken from first 100 chars of text if empty) |
| **Text** | Always | The master text. Per-account overrides possible (§6.4) |
| **Link** | Recommended for Pinterest/LinkedIn | Appended to X/FB text; Pinterest uses it as click-through destination |
| **Media** | REQUIRED for Instagram, YouTube (exactly 1 video), TikTok, Pinterest; optional for X/Facebook/LinkedIn | Images (jpg/png/gif/webp) and video (mp4/mov/webm/mkv); multi-select for carousels |
| **Date/time** | For scheduling | Leave empty + Schedule = saved as draft; fill + Schedule = queued |

Under the text box a live **character counter** shows. Diamonds (`…`) may appear
where trimming occurs.

### 6.2 Media rules that matter

- **YouTube = exactly 1 video.** No video → YouTube target is skipped with a note.
- **Shorts:** upload a vertical video ≤60s and put `#Shorts` in title/description.
- **X:** max 4 photos **or** 1 video per tweet. If you attach both, the video
  wins and images ride along in a follow-up reply.
- **Instagram carousel:** 2–10 items, same aspect ratio. Reels: vertical ≤3 min.
- **TikTok photo mode:** 1–35 images; otherwise 1 video (3s–10min).
- Max upload size per request: **100 MB**.

### 6.3 Selecting accounts ("Post to…")

Tick **any combination** of accounts across networks — e.g. 2 X profiles + IG +
LinkedIn company page. Publishing fans out to all of them concurrently.

### 6.4 Per-account customization

Click **Customize** under a ticked account to reveal:

- **Custom text** — replaces the master text for that account only
  (e.g. shorter hook for X, formal tone for LinkedIn, empty = use master).
- **First comment** — posted as the first comment (best home for Instagram
  hashtags, since caption links aren't clickable, and for overflow text).

Character usage `used/limit` is shown per account while customizing.

### 6.5 "Check limits" — the pre-flight report

Before publishing, click **Check limits**. For every selected account you get:

- Green `OK`, or red `NEEDS ATTENTION` with concrete errors, plus
- **Adaptations** — automatic remedies Solomon will apply, e.g.:
  - `will post as 3-tweet thread`
  - `will trim + append link/first-comment`
  - `only first 4 will be attached`
  - `requires image/video… target will be SKIPPED`

Fix red items (add media, add title/board, shorten) or accept the remedy.

### 6.6 ✨ Variants (AI captions)

1. Write (or paste) your text.
2. Click **✨ Variants** — uses the first selected account's network (or X by default).
3. You get 3 tones — **professional / casual / punchy** — each with a hashtag set
   tuned to that network (8 tags for IG/TikTok, 3 elsewhere).
4. Click **Use this** to load a variant into the editor.
5. The source is shown: `offline-templates` (built-in, always works) or
   `llm:<model>` when an AI key is configured (see DEV manual §5).

### 6.7 ⚡ Best time & Auto-schedule

- **⚡ Best time** — fetches the top 3 upcoming slots for your selected networks
  (defaults per network, boosted by YOUR past engagement — §11) and fills the
  date/time box with #1. Alternatives are listed with reasons and scores.
- **Auto-schedule** — submits the post with the best slot chosen automatically.
- **Schedule** — queues at your manually chosen date/time.
- **Post now** — publishes within seconds (statuses appear in Queue).

## 7. Queue tab — tracking every post

Each card shows:

- **Post status badge:** `draft` · `scheduled` · `published` (all targets ok) ·
  `partial` (some failed/skipped) · `failed` (none succeeded).
- Title, text, link, scheduled time, media count, **delete** button.
- **Per-target badges:** `network account: status` plus either the network post
  ID/note or the first 80 chars of the error, e.g.
  `instagram @acme.ig: skipped — instagram: requires image/video but post has none…`

Threads show comma-joined IDs (`demo_twitter_…_0,demo_twitter_…_1,…`).

### 7.1 Calendar view — see everything, move things

The **Calendar** tab shows a month grid (Monday-first) with a dot per post per
day: green = published, amber = scheduled/partial, red = failed, grey =
unscheduled/natively-posted. Dead-dot days are dimmed native posts (see §8).

- **Navigate:** ‹ Prev / Next ›, **Today**. Click any day to see its posts.
- **Move a scheduled post:** drag it onto another day (desktop) — it keeps its
  time of day — or set a new date/time in the day panel and click **move**.
  Moving into the past asks for confirmation first: the backend publishes
  anything due within ~15 seconds.
- **Schedule a draft:** drag it from the **Unscheduled** tray onto a day
  (lands at 09:00, keeping nothing else).
- **Empty day:** clicking one jumps to Compose with that date prefilled at 09:00.
- **Published posts don't move** (the past happened) — they offer **duplicate**
  (loads text/targets into the composer as a draft; tweak the text and
  re-attach media, since networks flag identical reposts as spam) and **delete**.

### 7.2 Deleting posts — what really happens

- **Scheduled / draft:** deleted forever from Solomon (nothing was published yet).
- **Published:** removed from Solomon but **kept in Analytics with a `deleted`
  badge** — your history stays intact. Network copies are NOT touched
  (like Buffer/Hootsuite, Solomon doesn't un-publish): delete those natively
  on each network if needed.
- Deleting a post also untangles it from evergreen pools and frees media files
  no other post uses.

## 8. Analytics tab — measuring results

1. Open **Analytics**, click **↻ Refresh stats** (pulls latest numbers for every
   published target).
2. Four totals: **views · likes · comments · shares**, plus a per-target table
   (network, account, the four metrics).
3. Rows tagged `demo` are sample data from demo tokens; live rows come from the
   network APIs (Instagram insights are live; others need extra scopes — see
   DEV manual §8).

Your engagement history feeds back into Best-time suggestions (§6.7).

### 8.1 Outside Solomon — posts you made natively

Anything you posted directly on the networks (phone app, native scheduler)
shows up here too: each **↻ Refresh stats** (plus an hourly background pass)
discovers native posts per account — 30-day backfill, stats frozen 20 days
after publishing, shown in their own **Outside Solomon** section with an
`outside` badge and dimmed dots on past calendar days. They're read-only
(no reschedule — they're the past) and excluded from app totals, but
**included in Best-time suggestions**, because your real history makes them
smarter. Notes: TikTok native discovery needs an audited app (skipped with a
note); X metrics need a paid tier; demo accounts show realistic sample native
posts.

## 9. Evergreen tab — bulk import & auto-recycling

### 9.1 Bulk CSV import

Upload a `.csv` with header (order doesn't matter, extra columns ignored):

```csv
title,content,link,scheduled_at,accounts
"Tip 1","Post about morning routines","https://example.com","2026-09-10 09:00","twitter;@acme"
"Tip 2","Second tip, no link or time","","","linkedin"
```

- `content` (or `text`) is required; rows without it are skipped.
- `scheduled_at` accepts RFC3339 (`2026-09-10T09:00:00+02:00`) or
  `YYYY-MM-DD HH:MM`. Empty = draft (or spaced scheduling with auto-import).
- `accounts` is a `;`-separated list matching **account names or network names**
  (case-insensitive): `"twitter"` = all X accounts; `"twitter;Acme Corp"` = all
  X accounts + the account named Acme Corp. Rows matching nothing are skipped
  with an error naming the row.
- Two upload buttons: **Standard import** and **Import + auto-schedule**
  (schedules rows 3 hours apart starting next hour).
- The result panel reports `Created N, skipped M` plus per-row errors.

### 9.2 Evergreen rules (recycle top posts forever)

1. **New rule** → name it (e.g. `weekly tips`), set **every N hours**.
2. Tick **pool posts** (existing posts to rotate through) and **accounts** to
   republish to.
3. **Save rule** — first run is scheduled N hours out; the 60s background worker
   republishes the next pool post (round-robin) as a brand-new post each time,
   copying text/title/link/media.
4. Rules list shows next run time; **remove** stops a rule (history is kept).

## 10. Network reference (limits, requirements, remedies)

| Network (API) | Max chars | Images | Video | Must have | Title? | Over-limit remedy |
|---|---|---|---|---|---|---|
| **X/Twitter** (X API v2) | 280 | 4 **or** 1 video/GIF | ≤140s, ≤512MB | — | no | **Auto-split into numbered thread** `(1/n)`; video wins over images |
| **Facebook** (Graph v21) | 63,206 | ≤10 (album) | 1, ≤4h, ≤10GB | — | no | auto-trim; extra images noted |
| **Instagram** (IG Graph) | 2,200 | ≤10 carousel | 1 reel ≤3min | **media required** | no | trim; **no-media → skipped** |
| **YouTube** (Data v3) | 5,000 desc / 100 title | 0 (thumbnail separate) | **exactly 1** ≤12h | **video + title** | **yes** | missing video/title → skipped |
| **TikTok** (Posting v2) | 2,200 | 1–35 photo mode **or** 1 video | 3s–10min | **media required** | no | no-media → skipped |
| **LinkedIn** (Posts API) | 3,000 | ≤9 | 3s–15min | — | no | **trim + overflow to first comment** |
| **Pinterest** (API v5) | 800 desc / 100 title | ≤5 | ≤15min | **media + board_id** | auto* | trim; auto-title from first 100 chars; link recommended |

\* Pinterest title auto-filled from the first 100 characters when empty.

Universal rules: unknown network → error; media over-count → first-N kept with a
note; anything a target can't satisfy becomes `skipped` with a plain-English
reason while sibling targets still publish (post becomes `partial`).

## 11. Background automation (what runs while you sleep)

| Worker | Every | Does |
|--------|-------|------|
| Due-post publisher | 15 s | Publishes posts whose time has come |
| Evergreen runner | 60 s | Republishes due evergreen rules |
| Token refresher | 1 h | Refreshes tokens expiring within 7 days |

No action needed — but the backend must be running (`make dev-bg`, or `go run .`)
for workers to fire.

## 12. Going live per network (OAuth, step by step)

General flow (same for all):

1. `cp backend/.env.example backend/.env` and fill your app's client ID/secret.
2. Restart the backend.
3. In Solomon: Accounts → select network → **Get OAuth URL** → approve in popup.
4. Provider redirects to `OAUTH_REDIRECT_BASE` with `?code=…`.
5. Exchange the code for tokens (see per-network docs below), then **Save account**
   with the access token (+ refresh token if given) and Extra IDs.
6. Test with **Post now** to a single demo-safe account first.

Where to register apps & what to ask for:

| Network | Developer console | Scopes / permissions | Token notes |
|---|---|---|---|
| X | developer.x.com → Project + OAuth 2.0 PKCE | `tweet.read tweet.write users.read offline.access` | Exchange `POST api.twitter.com/2/oauth2/token`; media upload is chunked (`upload.twitter.com`), polls via `poll` object |
| Facebook | developers.facebook.com → FB Login | `pages_show_list pages_read_engagement pages_manage_posts` | Use a **Page** token (`page_id` in Extra); multi-photo = unpublished `/photos` + `attached_media`; reels = `/video_reels`; insights need `read_insights` |
| Instagram | same FB app + IG Business/Creator linked to the Page | `instagram_basic instagram_content_publish pages_show_list` | Needs `ig_user_id`; **media must be a public URL** → set `PUBLIC_MEDIA_BASE_URL` (e.g. your domain or an ngrok URL); carousels = children with `is_carousel_item=true` |
| YouTube | console.cloud.google.com → OAuth client | `https://www.googleapis.com/auth/youtube.upload` (+ `yt-analytics.readonly` for stats) | Upload is resumable `videos.insert`; set privacy/category; Shorts = vertical ≤60s + `#Shorts` |
| TikTok | developers.tiktok.com → Login Kit | `user.info.basic video.upload video.publish` (+ `video.list` for stats, audited app) | Local files use `FILE_UPLOAD` init→chunks→publish; honor privacy/duet/stitch flags |
| LinkedIn | developer.linkedin.com | `openid profile w_member_social` (+ org scopes for company pages) | `author_urn` in Extra; send `LinkedIn-Version: 202401` header; programmatic refresh needs a partner app |
| Pinterest | developers.pinterest.com | `boards:read boards:write pins:read pins:write` | `board_id` in Extra (**required**); local files via multipart, URLs via `media_source.image_url` |

## 13. Troubleshooting & FAQ

| Symptom | Cause → Fix |
|---|---|
| `bind: address already in use` | Old server still running → `make stop`, then start again |
| Frontend shows network error / empty lists | Backend not running → `make dev-bg`, then `make health` (expect `{"ok":true}`) |
| Upload fails | Wrong field (must be `files`), unsupported extension, or >100 MB → use jpg/png/gif/webp/mp4/mov |
| Target `skipped`: requires image/video | Expected for IG/YT/TikTok/Pinterest without media → attach media or untick the account |
| Target `failed` with `board_id required` | Pinterest account missing `board_id` in Extra → edit by re-adding the account |
| YouTube `exactly 1 video required` / `requires a Title` | Attach one video + fill Title |
| X posted 3× but I wrote once | Your text exceeded 280 chars → it threaded by design; shorten or set per-account Custom text |
| Suggestions all "best-time default" | No analytics yet → publish + Refresh stats to unlock personalized boosts |
| Analytics row empty / `demo` tag | Demo token → sample numbers; connect a live token for real metrics |
| Token refresh error (LinkedIn) | Needs approved partner app → reconnect manually via OAuth URL |
| CSV row skipped: no matching accounts | `accounts` column names must match account names/networks (case-insensitive, `;`-separated) |
| Evergreen rule never runs | Backend must be running; first run is `interval_hours` after creation |
| `no expiry` badge | Account predates expiry tracking → hit ↻ Refresh tokens once |
| Moved post published instantly | You dropped it on a past date/time — anything due publishes within ~15s; the UI asks first, but confirming means now |
| "only draft/scheduled can be rescheduled" | Published history is fixed → use **duplicate** on the calendar day panel |
| Outside section empty | No native posts in the last 30 days, or the network needs extra setup (TikTok audited app, X user ID in External ID, board/author IDs in Extra) |
| Blank page, or an error about `map`/`null` | Stale frontend build talking to an old backend → hard-refresh the browser (Cmd/Ctrl+Shift+R); if it persists, rebuild + restart: `cd frontend && npm run build`, then restart the backend |
| `make stop` leaves a port busy | Kill by port directly, then start again: `lsof -ti:8080 \| xargs kill -9` (API) and `lsof -ti:5173 \| xargs kill -9` (web UI) |
| Lost data? | Restore `backend/solomon.db` + `backend/uploads/` from backup |

**Can I post different text per network in one go?** Yes — tick the accounts,
then **Customize** each one's text (§6.4).

**Can I reuse one image across networks?** Yes — attach once; every target uses
the same files (evergreen copies reference the same files too).

**Is there a mobile app?** No — but the web UI is responsive; open the same URL
on your phone on the same network.

## 14. Glossary

- **Account** — one connected profile/page/channel on one network.
- **Master post / Post** — your canonical title+text+link+media.
- **Target** — one instruction "publish this post to this account", optionally
  with custom text/first comment; has its own status and network post ID.
- **Thread** — an X reply-chain created from over-280-char text.
- **Adaptation** — an automatic fix (split/trim/skip) previewed by Check limits.
- **Evergreen rule** — a schedule that endlessly republishes a pool of posts.
- **Snapshot** — one analytics pull (views/likes/comments/shares) for a target.

# Architecture

How the template is put together, and the reasoning behind the load-bearing
decisions. Read this before reshaping the domain model.

## The one-binary shape

`cmd/app/main.go` is a one-liner into `app.Run()` (`internal/app/app.go`).
`Run` loads `.env`, parses `config.json` (env vars override payment/OAuth/
analytics values), loads i18n bundles and legal markdown, registers the
consolidated migration, then starts PocketBase. Everything — HTTP server,
database, file storage, mailer, admin UI (`/_/`), cron — lives in that one
process. A deploy is: replace binary (or container), restart.

PocketBase is used as a *framework*, not a backend-as-a-service:

- **All HTTP routes are ours**, wired in `registerRoutes`; handlers live in
  `handlers_*.go` split by domain (`auth`, `create`, `settings`, `overview`,
  `upload`, `export`, `payment`, `pages`, `seo`).
- **The public record API is closed.** The migration nulls every collection's
  API rules (superuser-only). Handlers enforce ownership and tier gating in
  Go. Do not move access control into PocketBase rules — you'd have two
  systems to keep consistent.
- What PocketBase buys us: SQLite + migrations, auth records + password
  reset mail, file storage (local or S3) with thumbnails, the admin UI, and
  structured logs.

## Domain model

```
users (auth)  1 ──< events  1 ──< prompts  1 ──< uploads
```

- **users** — hosts. `tier` (`free`/`standard`/`premium`) drives feature
  gating; `signup_lang`/`signup_country`/`auth_provider` are for analytics
  and lifecycle email; `marketing_opt_out` is consumed by the (docs-only)
  retargeting module.
- **events** — one per party/wedding/venue. Carries `title`, `event_date`,
  `lang`, `design_id` (palette), `single_qr_mode` and the paid guest-flow
  toggles.
- **prompts** — the photo tasks. *Every* upload binds to a prompt, even in
  products without visible prompts (a plain gallery = one prompt). This
  keeps the pipeline uniform: naming in ZIP exports, per-prompt galleries,
  the free-tier one-photo cap, single-QR rotation — all hang off prompts.
- **uploads** — guest submissions. `image` is the original; `display` an
  optional browser-friendly JPEG rendition (HEIC transcodes); `guest_name`
  optional attribution.

Cross-collection references are plain text ID fields, not PocketBase
relations — handlers join manually and the record API is closed, so relation
expansion would buy nothing.

**Migrations:** one consolidated file (`migrations/01_collections.go`).
While your product is unlaunched, edit it and delete `pb_data/` — no
migration archaeology. Once real data exists, switch to additive numbered
migrations (add fields as optional so no backfill is needed). Note that
PocketBase pre-creates the `users` auth collection, so the migration
*extends* it idempotently rather than creating it.

## Request path for HTML pages

1. Router-level middleware `attachAuthFromCookie` hydrates `e.Auth` from the
   `pb_auth` cookie (PocketBase's own auth middleware only runs on routes
   bound with `apis.RequireAuth()`, which plain HTML routes aren't).
2. `applyLangPreference` makes the nav language switcher sticky
   (`?setlang=` → cookie → redirect on later bare-URL visits).
3. The handler loads records, enforces ownership, and calls
   `renderWithBase(e, "page", data)`.
4. `renderWithBase` (templates.go) resolves the language from the URL
   prefix, clones the cached parsed template, rebinds `T`/`THTML` to
   per-request closures, and injects the standard data (AppName, Auth,
   canonical/hreflang URLs, PostHog config, …).

Template caching means a process restart shows new view content; there is no
hot reload.

## Localisation is baked into routing

The default language (`en`) serves at the bare path; every other supported
language is mounted at `/<lang>/…` by `registerLocalisedGet/Post`. Sitemap
and `hreflang` derive from the same `i18n.SupportedLangs` list, so **adding a
language is**: append to `SupportedLangs`, add `LangNames`/`LangLocales`
entries, drop a complete `data/locales/<lang>.json` (the parity test fails
until it's complete), add a flag SVG. Routes, sitemap, hreflang pick it up.

Guest pages are different on purpose: guests follow the **event's** language
(plus a `?lang=` pin carried in printed QR URLs), the site-wide switcher is
suppressed there (`GuestPage` flag), and user-specific pages are `noindex`
and not part of the sitemap.

Webhooks and the OAuth callback mount once at their canonical path (Google
matches the registered redirect URI exactly; the language survives in a
cookie).

## Guest flow and the two QR modes

An event is distributed in one of two modes (`single_qr_mode`, switchable
any time):

- **Cards mode**: one printed card per prompt, each QR points at
  `/e/{id}/{promptID}`.
- **Single-QR mode**: one poster QR points at `/e/{id}`; the dispatcher
  hands each scanner the next prompt — sticky per browser via a compact
  bitset cookie (500 prompts ≈ 63 bytes), rotating by global `show_count`
  with coverage-first selection (`pickPrompt`).

Both land on the same upload handler. The free-tier cap (one photo per
prompt), the one-upload-per-guest soft lock, and name collection are
enforced server-side; the lock is a cookie, so it's a *soft* lock by design
(owners bypass it when authenticated).

**Unknown GET paths fall through to the landing page** (Go ServeMux treats
the `/` pattern as a catch-all). Handlers therefore render their own 404s
for real not-found cases.

## Tier gating

`helpers.go` is the single home of monetisation logic:

- `getUserTier` reads the user's tier against `config.json` tiers.
- `isPaidTier` gates host-side features (bare QR downloads).
- `eventOwnerPaid` resolves the *owner's* tier from an event — used on
  unauthenticated guest routes (multi-photo, guest ZIP, lock, name field).
- Paid per-event toggles only take effect while the owner is paid, so a
  downgrade quietly disables them without mutating stored settings.

Payments: `handlePayment` creates a Lemon Squeezy hosted checkout with
`user_id`+`tier` in `custom_data`; the webhook (`lemon.go`) verifies the
HMAC signature and upgrades the user. The pricing-variant machinery
(PostHog feature flags swap displayed prices and `?variant=` on the checkout
link) ships but is dormant until you define `pricing_variants` in
config.json.

## Print module

`generateCardsPDF` / `generatePosterPDF` build a `printJob` (event metadata,
palette, per-prompt QR URLs) and shell out to `typst compile`
(`pdf_typst.go`). QR PNGs are pre-generated server-side into a per-job temp
dir under `pb_data/typst/`. The poster and cards share the same palette
(`designs.go`), so all print material matches the guest pages.

`classic.typ` also has `preview-front`/`preview-back` single-card modes used
by `RenderCardPNG` (the `cmd/preview-cards` marketing-image CLI). Layout
guarantee: text blocks are auto-fitted (font size steps down until the block
fits), so no host input can overflow a card or the poster — keep that
guarantee when editing templates.

`config.json`'s `app_url` is what ends up inside printed QR codes — it must
be the public URL in production.

## Design system

`designs.go` defines five palettes (primary/secondary/accent/background/
text) used by **both** the web guest pages (inline styles) and the Typst
print templates (via `printDesign`). The site chrome itself is themed by CSS
custom properties at the top of `main.css`. See DESIGN.md.

## Media pipeline

Uploads are sniffed by magic bytes (`detectImageFormat`) — broad allowlist
(JPEG/PNG/GIF/WebP/BMP/TIFF/HEIC/HEIF) because phones emit all of them.
HEIC/HEIF (iPhone default) can't render in browsers, so a background
goroutine (bounded to 2 concurrent WASM decodes) transcodes a JPEG into the
`display` field; galleries prefer `display` and fall back to `image`. The
ZIP export streams through PocketBase's filesystem abstraction so it works
identically on local disk and S3.

## Config and secrets

- `config.json` (path override: `CONFIG_PATH`) holds public/product config:
  app name/url, tiers, pricing variants, S3, PostHog key.
- Secrets come from env only (`LEMON_SQUEEZY_*`, `GOOGLE_CLIENT_*`); `.env`
  is loaded for dev but **existing env vars always win**, so production
  deploys can override.
- Every optional integration is enabled by the presence of its credentials
  and cleanly absent otherwise (Google button hidden, checkout returns
  "not configured", no PostHog script tag).
- `PB_SUPERUSER_EMAIL/PASSWORD` bootstrap an admin on first boot.

## Testing strategy

- **Go unit tests** pin the tricky pure logic: guest-cookie bitset
  round-trips, ZIP streaming through the storage abstraction (regression
  test for the S3 empty-archive bug), redirect safety, locale parity,
  lang-preference middleware, Typst renders (skipped when typst is absent).
- **Playwright** (`tests/`, 54 tests) drives the current product through the
  whole funnel, including localized pages, photo/video uploads, tier upgrades via the superuser API
  (`upgradeToPaid` helper) and multipart upload edge cases.
- CI (`.github/workflows/build.yml`) runs both before building/pushing the
  Docker image.

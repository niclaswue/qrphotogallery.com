# QR Photo Gallery

This repository is the qrphotogallery.com product built from the original
QR-photo template. It is deliberately focused on one job: share one QR code
and collect every guest photo and video in original quality.

Product defaults:

- Personal: €19 one-time, for private/non-commercial use
- Commercial: €29 one-time, for businesses, professionals and client work
- 100 GB per gallery, common photo/video formats, up to 2 GB per file
- EU-hosted storage, one-year availability and automatic retention cleanup
- no app or guest account; batch uploads from any modern phone browser

The internal `events → prompts → uploads` pipeline remains intact, with a
single hidden prompt acting as each gallery's upload bucket.

## Template heritage

A production-proven template for **QR-code event media businesses**: guests
scan a printed QR code at an event, get a page in their phone browser, and
upload photos — no app, no account. The host manages everything from a web
dashboard and pays a one-time fee for the full feature set.

Extracted and generalised from [PhotoChallenge Wedding](https://photochallenge.wedding)
(min-pcw), which runs this exact stack in production. The template is a
**fully runnable app**, not a scaffold: clone it, `go run` it, and you have a
working generic product ("QR Photo App") to reshape into your next idea.

Products this template is designed to become (see [docs/ADAPTING.md](docs/ADAPTING.md)
for concrete per-idea guides):

- QR photo challenge for parties / weddings / any celebration *(≈ what it is out of the box)*
- Simple QR photo gallery ("drop your photos here")
- QR photo bingo with teams
- QR audio guestbook
- QR live photo slideshow

## Stack

- **Go 1.25** + [PocketBase v0.37](https://pocketbase.io/) as a framework
  (pure-Go SQLite → single static binary, `CGO_ENABLED=0`)
- HTML pages: Go `html/template` in `views/`, cached page templates with
  per-request i18n closures — no frontend framework, a sprinkle of vanilla JS
  and htmx
- Print material: [Typst](https://typst.app/) templates in `templates/print/`
  rendered via subprocess (`brew install typst`)
- Payments: Lemon Squeezy one-time payments (checkout + webhook)
- Auth: email/password + optional Google OAuth2 (env-gated)
- Analytics: PostHog (optional, consent-gated; also drives pricing-experiment
  feature flags)
- Tests: Go unit tests + a Playwright browser suite covering the whole funnel
- Deploy: Docker → GHCR → VPS with Caddy (TLS), Watchtower (auto-deploy) and
  nightly S3 backups — see [docs/DEPLOY.md](docs/DEPLOY.md)

## Quickstart

```bash
brew install typst                    # print PDFs need the typst CLI on PATH
cp example.env .env                   # optional — everything runs without secrets
go run ./cmd/app serve                # http://localhost:8090
```

That's it. Register an account, create an event, print the PDF, scan the QR
with your phone (use your LAN IP as `app_url` in config.json to test from a
real phone), upload a photo, download the gallery ZIP.

```bash
go test ./...                                    # Go unit tests
cd tests && npm install && npm test              # Playwright suite (server must be running)
go run ./cmd/preview-cards                       # render marketing card previews (needs pdftoppm, cwebp)
```

## Spinning up a new business from this template

Work through this checklist top to bottom. Steps 1–4 get you a rebranded
running product; the rest is per-idea product work.

1. **Clone & rename**
   ```bash
   git clone <this-repo> my-new-product && cd my-new-product
   rm -rf .git && git init
   # module path: replace everywhere
   grep -rl 'github.com/niclaswue/template-qr-photo' --include='*.go' . go.mod \
     | xargs sed -i '' 's|github.com/niclaswue/template-qr-photo|github.com/YOU/my-new-product|g'
   ```
2. **Brand it** — one place each:
   - `config.json`: `app_name`, `app_url`, `support_email`, tier prices
   - `data/locales/en.json` + `de.json`: all user-facing copy (the
     locale-parity Go test keeps the bundles honest)
   - `pb_public/static/css/main.css`: the design tokens at the top
     (`--color-primary` etc.) — see [docs/DESIGN.md](docs/DESIGN.md)
   - `pb_public/static/img/og-default.jpg` + `favicon.svg`
   - `data/legal/*.md`: imprint / privacy / refund (placeholders with TODOs)
3. **Decide the domain model** (see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)):
   the generic model is `events` → `prompts` → `uploads`. Most ideas map onto
   it directly (a plain gallery is an event with one prompt in single-QR
   mode; bingo is prompts + a teams collection). Only reshape the schema if
   the mapping genuinely doesn't fit — edit `migrations/01_collections.go`
   freely while unlaunched, it's a single consolidated migration.
4. **Wire the money & services** (each is optional and off until configured):
   - Lemon Squeezy: create products, set `LEMON_SQUEEZY_*` env vars
   - Google OAuth: `GOOGLE_CLIENT_*` env vars
   - PostHog: project key in `config.json` or `POSTHOG_KEY`
5. **Build the product-specific parts** — this is where
   [docs/ADAPTING.md](docs/ADAPTING.md) has a concrete section per idea, plus
   guides for porting the modules that deliberately stayed behind in min-pcw
   (retargeting emails, ops toolkit, content-marketing/SEO machinery, the
   card/poster designer UIs, audio/video media kinds).
6. **Deploy** — [docs/DEPLOY.md](docs/DEPLOY.md): one VPS, Docker Compose,
   Caddy for TLS, GitHub Actions builds the image, Watchtower auto-deploys,
   nightly S3 backups.

## What's included (and what's docs-only)

**Working code, wired in:**

| Module | Where |
|---|---|
| Auth (email + Google OAuth, PKCE) | `handlers_auth.go`, `oauth_google.go` |
| Event / prompt / upload domain | `event.go`, `handlers_*.go`, `migrations/` |
| Guest flow (per-prompt QR + single-QR rotation) | `handlers_upload.go` |
| Image pipeline (HEIC→JPEG rendition, thumbs, ZIP export) | `imageconv.go`, `zip.go` |
| Print module (card deck + poster PDFs, live-preview-quality) | `pdf*.go`, `templates/print/` |
| Payments + tier gating + pricing A/B machinery | `handlers_payment.go`, `lemon.go`, `helpers.go` |
| i18n (URL-prefix routing, hreflang, sitemap; en+de shipped) | `internal/i18n/`, `handlers_seo.go` |
| Base design system (5 palettes shared by web + print) | `designs.go`, `main.css` |
| Ops basics (GeoIP country on signup, locked API rules, admin thumbs) | `geoip.go`, `migrations/` |

**Docs-only** (proven in min-pcw, port when needed — instructions in
[docs/ADAPTING.md](docs/ADAPTING.md)): retargeting/lifecycle email, the
`agent_ops/` read-only business-ops toolkit, KPI `stats` CLI, SEO content
pages & guide-only languages, the interactive card/poster designer UIs,
per-event prompt translations, review collection, audio/video media kinds.

## Layout

```
cmd/app/                  main entry — calls app.Run()
cmd/preview-cards/        CLI: render card preview WebPs for marketing pages
internal/app/             bootstrap, routes, handlers, PDF, payments, OAuth, i18n glue
internal/i18n/            translation loader and lang-aware URL helpers
migrations/               ONE consolidated migration — evolve freely pre-launch
templates/print/          Typst print templates (classic.typ cards + poster.typ + _shared/)
views/                    Go html/template pages (base.html + one file per page)
data/locales/             en.json / de.json translation bundles (parity-tested)
data/legal/               imprint / privacy / refund markdown (TODO placeholders)
data/fonts/               fonts bundled into the Typst PDFs
pb_public/static/         css/js/img/fonts served at /static; pb_data/ is gitignored
tests/                    Playwright browser suite (79 tests, full funnel)
deploy/, Dockerfile       VPS deploy: compose + Caddy + Watchtower + S3 backup
docs/                     ARCHITECTURE, ADAPTING, DEPLOY, DESIGN
```

## Opinionated defaults you get for free

- **Anonymous-first create flow**: visitors build their event *before*
  registering; the filled form survives the sign-up round-trip via cookie.
  This converts far better than auth-first.
- **All data access through handlers**: PocketBase's record API is locked to
  superusers by migration; ownership and tier gating are enforced in Go, not
  in API rules.
- **Paid toggles degrade gracefully**: per-event paid settings only take
  effect while the owner is on a paid plan — downgrades quietly disable them
  without touching stored settings.
- **Preview == print**: the poster/cards the host sees are the exact Typst
  render that comes out of the printer.
- **Locale parity is a test**: a missing translation key fails `go test`.
- **The whole funnel is a browser test**: register → create → print →
  guest upload → gallery → ZIP → tier gating, in 79 Playwright tests.

# QR Photo Gallery

A focused QR-code event gallery: the host creates one gallery, shares one QR
code, and guests upload photos and videos from their browser. Every upload
appears in the same shared gallery. There are no prompts, themes, card decks,
or guest accounts in the product UI.

## Product at a glance

- one gallery URL, one QR PNG, and one printable QR poster
- batch photo and video uploads from current phone browsers
- a flat, shared gallery for guests and a unified host dashboard
- originals retained at full quality and downloadable as a ZIP
- 100 GB per gallery, up to 2 GB per file, available for one year
- English and German routes and copy
- EU-compatible local or S3-backed file storage

The paid offers differ by usage license, not by hiding the core gallery:

- **Personal — €19 once:** private parties, weddings, birthdays, and family
  events
- **Commercial — €29 once:** business and client events, with uploader-name
  collection, public-download controls, and priority support

New accounts get a functional one-file preview before payment.

## Stack

- Go 1.25 and PocketBase v0.37 (pure-Go SQLite; `CGO_ENABLED=0`)
- Go `html/template` views with a small amount of vanilla JavaScript
- Typst for the printable single-QR poster
- Lemon Squeezy one-time payments
- optional Google OAuth and PostHog
- Go unit tests and a Playwright end-to-end suite

## Quickstart

```bash
brew install typst                    # needed for the printable poster
cp example.env .env                   # optional; the app runs without secrets
go run ./cmd/app serve                # http://localhost:8090
```

Register, create a gallery, open its shared URL, and upload a photo. To scan
the QR code from a real phone, set `app_url` in `config.json` to an address
that phone can reach before generating the poster.

```bash
go build -o app ./cmd/app
go vet ./... && go test ./...

cd tests
npm install
npm test                              # requires the app to be running
```

## Architecture

The customer-facing model is deliberately small:

```text
host → event gallery → uploads
```

Internally, the inherited `prompts` collection remains as a deletable storage
layer: each event gets exactly one hidden upload bucket. It is never rendered
to hosts or guests. This preserves compatibility with existing databases while
the product behaves as a simple gallery everywhere.

Key routes:

| Route | Purpose |
|---|---|
| `/create` | gallery name and optional event date |
| `/overview/{id}` | host dashboard, one QR, all uploads, and settings |
| `/e/{id}` | combined guest uploader and shared gallery |
| `/poster/{id}` | printable single-QR PDF |
| `/qr-image/{id}` | bare gallery QR PNG |
| `/download/{id}` | host ZIP containing original uploads |

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for request flow, storage,
tier gating, and migration details.

## Adapting and branding

The main product settings live in a few obvious places:

1. `config.json`: app name, public URL, support address, and tier prices.
2. `data/locales/en.json` and `de.json`: all customer-facing copy. Tests
   enforce locale parity and verify every referenced key exists.
3. `pb_public/static/css/main.css`: brand colors, type, spacing, and shared
   components. See [docs/DESIGN.md](docs/DESIGN.md).
4. `pb_public/static/img/hero-gallery.webp`, `og-default.jpg`, and
   `favicon.svg`: product imagery and metadata.
5. `data/legal/*.md`: imprint, privacy, and refund content.

The consolidated pre-launch schema is in `migrations/01_collections.go`.
PocketBase pre-creates `users`; the migration extends it idempotently. Before
launch, edit the consolidated migration and recreate `pb_data/`. Once real
data exists, use additive numbered migrations.

For product variants such as a slideshow or audio guestbook, see
[docs/ADAPTING.md](docs/ADAPTING.md).

## Repository layout

```text
cmd/app/                  application entry point
internal/app/             routes, handlers, domain logic, print, payments, i18n glue
internal/i18n/            locale loader and language-aware URL helpers
migrations/               consolidated PocketBase schema
templates/print/          fixed single-QR Typst poster
views/                    Go html/template pages
data/locales/             English and German bundles
data/legal/               localized legal documents
pb_public/static/         CSS, JavaScript, fonts, and images
tests/                    Playwright browser suite
deploy/, Dockerfile       production deployment
docs/                     architecture, adapting, deployment, and design guides
```

## Product invariants

- **One QR is canonical.** `/e/{id}` is both uploader and gallery; old
  per-prompt or library URLs only redirect there.
- **Uploads are flat.** Neither the host dashboard nor guest gallery exposes
  internal buckets.
- **Originals stay original.** Browser renditions may be generated for HEIC,
  while ZIP exports use the uploaded originals.
- **Access control lives in handlers.** The public PocketBase record API is
  locked to superusers.
- **Translation failures are tests.** Missing referenced locale keys fail the
  Go suite instead of becoming `[key.name]` markers in production.
- **The whole funnel is exercised.** The browser suite covers registration,
  gallery creation, QR/poster output, uploads, gallery viewing, ZIP downloads,
  localization, tier controls, and route security.

# Architecture

The application is intentionally shaped around one product flow: create a
gallery, share one QR code, and collect every guest upload in one place.

## One process, one binary

`cmd/app/main.go` calls `app.Run()` in `internal/app/app.go`. Startup loads
configuration, translations, and legal content; registers the consolidated
migration and HTTP routes; then starts PocketBase. The HTTP server, SQLite
database, file storage, authentication, admin UI, cron jobs, and mailer all
live in that process.

PocketBase is used as a Go framework rather than as a public backend service:

- routes are registered in `app.go`, with domain handlers in
  `handlers_*.go`;
- all public collection API rules are locked to superusers;
- handlers enforce ownership and paid-feature access;
- PocketBase provides records, migrations, authentication, storage, mail,
  logs, and the admin UI.

## Data model

```text
users  1 ──< events  1 ──< prompts  1 ──< uploads
```

The customer-facing model is simply `host → gallery → uploads`. The middle
`prompts` collection is inherited storage plumbing: every new event gets
exactly one hidden record named `Gallery uploads`, and every file is attached
to it. No prompt text or prompt grouping appears in the UI, URLs, ZIP names,
or print output.

- **users** — hosts. `tier` is `free`, `standard`, or `premium`; signup
  metadata supports analytics and optional lifecycle work.
- **events** — galleries. Stores owner, title, optional date, guest language,
  the two Commercial controls, and creation timestamps.
- **prompts** — one hidden upload bucket per newly created event. Existing
  databases with older prompt records remain readable; all their uploads are
  flattened by event ID.
- **uploads** — original media plus optional guest name. HEIC/HEIF files may
  also receive a browser-friendly JPEG in `display`.

Cross-collection references are text IDs. Handlers join records explicitly,
which keeps the schema and deletion paths easy to understand.

The pre-launch schema is one consolidated migration,
`migrations/01_collections.go`. PocketBase creates the `users` collection
itself, so the migration extends it idempotently. Before launch it is fine to
edit this migration and recreate `pb_data/`; after launch, use additive
numbered migrations.

## HTML and translation flow

1. `attachAuthFromCookie` hydrates `e.Auth` for ordinary HTML routes.
2. `applyLangPreference` persists an explicit language choice.
3. The handler loads records, checks ownership or public availability, and
   calls `renderWithBase`.
4. Templates are cached per page **and language**. `T` and `THTML` are bound
   to that language before parsing, so a parser stub can never leak blank or
   bracketed copy into rendered pages.
5. `renderWithBase` adds common navigation, canonical and `hreflang` URLs,
   current user/tier data, and optional analytics configuration.

The default language is served at bare paths and other languages at
`/<lang>/…`. Adding a language means updating the supported-language metadata,
adding a complete locale JSON file and flag asset, and translating the legal
documents. Tests enforce locale parity and scan every template/handler locale
reference so values such as `[create.eyebrow]` fail CI.

Guest pages follow the gallery's stored language, with `?lang=` available for
QR links. They are `noindex` and omit the site language switcher.

## The one-QR guest flow

`/e/{id}` is the canonical public URL and the only URL encoded in QR output.

- `GET` renders the event title, multi-file uploader, and a flat list of every
  existing photo and video.
- `POST` validates a batch, saves it to the hidden bucket, and redirects back
  to `/e/{id}?uploaded=1#gallery`.
- The shared gallery supports a keyboard-accessible lightbox and, when the
  owner permits it, an originals ZIP download.
- Old `/e/{id}/library` and `/e/{id}/{promptID}` links redirect to the
  canonical page for backward compatibility.

There is no guest account, prompt selection, rotation cookie, completion
screen, per-prompt page, theme, or multiple-QR mode.

Unknown GET paths can reach the ServeMux `/` catch-all, so not-found cases in
real handlers must render explicit 404 responses.

## Host flow

`/overview/{id}` is the single host workspace. It contains:

- the canonical URL and one QR preview;
- QR PNG and printable poster downloads;
- upload count, storage usage, and expiry date;
- one flat media overview with original ZIP download and deletion;
- gallery language and Commercial controls;
- title/date editing and gallery deletion.

The old owner `/gallery/{id}` URL redirects to the uploads section of this
page.

## Plans and limits

Tier logic is centralized in `helpers.go`:

- **Free** is a functional one-file preview.
- **Personal (`standard`)** costs €19 once and unlocks repeat/batch uploads,
  100 GB total storage, originals ZIPs, QR PNG, and poster for private use.
- **Commercial (`premium`)** costs €29 once and provides the same complete
  gallery under a commercial license, plus uploader-name collection, public
  ZIP control, and priority support.

The server enforces 2 GB per file, 4 GB per request, 100 files per batch,
100 GB per gallery, and one year of availability. `eventOwnerPaid` is used on
public guest requests, where there is no authenticated host record. Commercial
settings only take effect while the owner has the Commercial tier.

Payments use a Lemon Squeezy hosted checkout. The app passes the user ID and
tier in checkout custom data; the webhook verifies its signature before
changing the account tier. Visiting a payment-success URL alone never grants
access.

## Media and storage

Upload validation uses magic bytes rather than trusting a filename or MIME
header. Supported still formats include JPEG, PNG, GIF, WebP, BMP, TIFF, HEIC,
and HEIF; supported video containers include common MP4/QuickTime, WebM, AVI,
MPEG, and 3GP variants.

HEIC/HEIF originals are retained. A bounded background converter may add a
JPEG rendition for browser display. The shared/owner galleries prefer that
rendition, while ZIP export streams the original file through PocketBase's
filesystem abstraction. The same code therefore works with local storage and
S3-compatible storage.

A daily retention job deletes expired gallery records and their stored files.
Public access also checks the one-year deadline synchronously, closing the gap
between expiry and the next cleanup pass.

## Print output

The print module has one job: generate an A4 poster containing the event name,
optional date, and canonical gallery QR. `pdf.go` builds the fixed print job;
`pdf_typst.go` invokes `typst compile` with `templates/print/poster.typ`.

The bare PNG endpoint uses the same canonical URL. `config.json`'s `app_url`
must therefore be the real public production origin before materials are
generated.

## Configuration

- `config.json` contains public product configuration: name, URL, prices,
  storage selection, and optional PostHog values.
- Lemon Squeezy and Google OAuth secrets come from environment variables.
- `.env` is loaded for development, but pre-existing environment variables
  win.
- Optional integrations disappear cleanly when not configured.
- `PB_SUPERUSER_EMAIL` and `PB_SUPERUSER_PASSWORD` can bootstrap an admin on
  first start.

## Testing

- Go tests cover locale completeness, template parsing, media sniffing, ZIP
  streaming, cookie/redirect safety, date formatting, retention behavior, and
  Typst output when Typst is installed.
- Playwright covers the full product funnel in English and German, including
  registration, anonymous create handoff, one-QR output, the combined guest
  page, decodable media rendering, repeat/batch uploads, ZIPs, Commercial
  controls, legacy redirects, and ownership/security boundaries.
- CI runs both suites before building the deployment image.

# AGENTS.md

Guidance for AI coding agents (and humans) working in this repository.
`CLAUDE.md` is a symlink to this file.

# QR Photo Gallery

A focused QR event gallery: guests scan one printed QR code, upload photos
and videos in their phone browser, and browse the flat shared gallery. The
host manages one QR, all uploads, downloads, and settings from one dashboard.
There are no prompts, themes, or card decks in the product UI.

Start with `README.md` (quickstart + adaptation checklist), then
`docs/ARCHITECTURE.md` (system design), `docs/ADAPTING.md` (product variants),
`docs/DEPLOY.md`, `docs/DESIGN.md`.

## Design principles

- Write simple, clean code. Functionality first.
- This is a template: prefer the boring, obvious solution — every reader is
  about to fork and reshape it.
- Keep modules deletable: one handler file + routes + view + locale keys.
- Prefer a clean rewrite over a hack: future-you pays for the shortcut.

## Stack

Go 1.25 + PocketBase v0.37 (pure-Go SQLite, build with `CGO_ENABLED=0`),
Go `html/template` views, Typst print PDFs (needs `typst` on PATH),
Lemon Squeezy payments, optional Google OAuth + PostHog, Playwright tests.

## Commands

```bash
go build -o app ./cmd/app            # build (package lives under cmd/app)
./app serve                          # dev server on :8090
go vet ./... && go test ./...        # unit tests (locale parity, zip, uploads, typst)
go test ./internal/app -run TestName

cd tests && npm install && npm test  # Playwright suite — needs ./app serve running
npx playwright test 03-gallery.spec.ts       # single file
```

## Architecture in five lines

- `internal/app/app.go` boots PocketBase and wires every route; handlers in
  `handlers_*.go` by domain; domain logic in `event.go`, gating in
  `helpers.go`.
- Collections `users` / `events` / `prompts` / `uploads` come from ONE
  consolidated migration (`migrations/01_collections.go`) — edit it freely
  pre-launch and delete `pb_data/`; the public record API is locked to
  superusers, all access control lives in the handlers.
- Localised routing: default lang at bare paths, others under `/<lang>/…`;
  templates are cached per page/language with `T`/`THTML` bound before parse;
  locale parity and referenced keys are test-enforced.
- Guest flow: `/e/{id}` is the combined unauthenticated uploader and flat
  gallery. Each event has one hidden prompt record only as a storage bucket.
- Print: `pdf.go` builds the fixed single-QR poster job; `pdf_typst.go` shells
  out to `typst compile` with `templates/print/poster.typ`.

## Things that bite

- Unknown GET paths fall through to the landing page (ServeMux `/`
  catch-all) — render explicit 404s in handlers.
- `app_url` in config.json ends up inside printed QR codes.
- PocketBase pre-creates the `users` collection; the migration extends it
  idempotently — keep that property.
- Template cache: restart the process to see view changes.
- Adding a locale key: add it to BOTH `en.json` and `de.json` or
  `go test` fails.

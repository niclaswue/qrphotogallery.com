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
- Core collections `users` / `events` / `prompts` / `uploads` come from the
  consolidated migration (`migrations/01_collections.go`); the isolated
  one-hour landing demo is added by `migrations/02_demo_galleries.go`. The
  public record API is locked to superusers and access control lives in the
  handlers.
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

## Production deployment (qrphotogallery.com)

Live as a Docker stack on the shared VPS **178.104.252.218** (Ubuntu 24.04),
under the `qrphotogallery` user (in the `docker` group; no sudo). A second,
pre-existing stack — **min-pcw** (serves `photochallenge.wedding`, owned by the
`pcw` user in `/home/pcw/min-pcw`) — shares this host. Both stay isolated except
two deliberately shared pieces: the **reverse proxy** and the **watchtower**.
Everything qrphotogallery lives in `~/qrphotogallery/` (config, compose, helper
scripts). Superuser is bootstrapped from `.env` at `https://qrphotogallery.com/_/`.

### Topology (why it's built this way)
Only one process can own :80/:443, and min-pcw's Caddy already does (auto
Let's Encrypt). So qrphotogallery runs **no Caddy of its own**:

```
Internet :443 ─▶ min-pcw-caddy (shared edge, network min-pcw_default)
                 ├─ photochallenge.wedding ─▶ app:8090      (min-pcw)
                 └─ qrphotogallery.com      ─▶ qrapp:8090   (qr-app)
```

Two shared-host traps this setup was built around (do not regress):
- **Service is named `qrapp`, never `app`.** Compose auto-adds the *service
  name* as a network alias on every attached network. On the shared
  `min-pcw_default`, an `app` alias would collide with min-pcw's own `app` and
  round-robin photochallenge.wedding traffic into the wrong app.
- **One watchtower for the whole host.** Two watchtowers delete each other on
  startup ("excess instance" cleanup). qrphotogallery runs no watchtower;
  `qr-app` carries `com.centurylinklabs.watchtower.enable=true` and the single
  `min-pcw-watchtower` (LABEL_ENABLE, host-wide) updates it — it pulls this
  private image with the shared `ghcr.io` token. Never start a second watchtower.

### CI/CD (this is "watchtower deployment from GitHub")
push to `main` → `.github/workflows/build.yml` (Go tests + full Playwright must
pass) → publishes private `ghcr.io/niclaswue/qrphotogallery.com:latest` →
`min-pcw-watchtower` polls every 60s and redeploys `qr-app`.
- GHCR pull auth: `~/.docker/config.json` for both `qrphotogallery` and `pcw`
  holds a read `ghcr.io` token (same `niclaswue` token; it can pull this image).
- The CI `test` job installs **typst** (poster endpoint 500s without it) — keep it.

### Operate (on the VPS, as `qrphotogallery`)
```bash
cd ~/qrphotogallery
docker compose pull && docker compose up -d          # manual redeploy
docker compose logs -f qrapp                          # app logs
docker exec min-pcw-caddy wget -qO- http://qrapp:8090/api/health   # edge→app
./add-caddy-block.sh        # add qrphotogallery.com to shared Caddy (see below)
./rollback-caddy-block.sh   # revert that Caddy edit
```
`add-caddy-block.sh` backs up min-pcw's Caddyfile, appends the
`qrphotogallery.com` + `www` blocks (from `caddy-qrphotogallery.snippet`),
`caddy validate`s, then graceful `caddy reload` — photochallenge.wedding never
restarts. Run it **only after DNS points at the box** (otherwise Caddy burns
Let's Encrypt failed-validation attempts). Edits go through a root container
because the Caddyfile is pcw-owned; never bind :80/:443 with a second proxy.

### DNS + TLS
Namecheap → qrphotogallery.com Advanced DNS: `A @ → 178.104.252.218`,
`A www → 178.104.252.218`; remove the parking/URL-redirect records, leave MX.
TLS issues automatically once DNS resolves here and the Caddy block is live.

### R2 object storage (staged, wire up later)
1. `config.json`: `"s3": {"enabled": true, "endpoint":
   "https://<accountid>.r2.cloudflarestorage.com", "bucket": "qrphotogallery",
   "region": "auto"}`.
2. `.env`: fill `S3_ACCESS_KEY_ID/SECRET`, `S3_REGION=auto`, `S3_ENDPOINT`,
   `S3_BUCKET` (placeholders already present).
3. `docker compose up -d`.
For nightly `pb_data` backups to R2, fill `BACKUP_S3_*` and add the `backup`
service (offen/docker-volume-backup) from `deploy/docker-compose.yml`.

Pocketbase
- Superuser: niclaswue@gmail.com / oW0mdwG92xMiiw037iOHQOzt74b6

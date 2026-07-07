# Deploying on a VPS

One small VPS (2 vCPU / 4 GB, e.g. Hetzner CX22) comfortably runs this
stack: the app is a single static binary with SQLite, so there is no
database server to operate. The pipeline is:

```
git push → GitHub Actions (tests → Docker image → GHCR)
                └── Watchtower on the VPS polls GHCR and restarts the app
```

## One-time VPS setup

1. **Provision** a Debian/Ubuntu VPS; install Docker + the compose plugin.
   Create a non-root user, disable password SSH.
2. **DNS**: point `yourdomain.com` (and optionally `staging.yourdomain.com`)
   at the server.
3. **Copy the deploy bundle** to the server:
   ```bash
   scp -r deploy/ you@server:~/app/
   cd ~/app
   cp .env.example .env          # fill in real secrets
   cp config.json.example config.json   # app_name, app_url=https://yourdomain.com, tiers
   ```
4. **Caddyfile**: replace the `:80` block with your domain block (template
   inside `deploy/Caddyfile`). Caddy provisions TLS automatically.
5. **Private GHCR pulls**: `docker login ghcr.io` with a read-only PAT so
   Watchtower can pull (it reads `~/.docker/config.json`).
6. **First start**:
   ```bash
   docker compose up -d
   ```
   Set `PB_SUPERUSER_EMAIL`/`PB_SUPERUSER_PASSWORD` in `.env` before first
   boot — that bootstraps the PocketBase admin at `https://yourdomain.com/_/`.

## What runs (deploy/docker-compose.yml)

| Service | Job |
|---|---|
| `app` | the Go binary; image from GHCR; `pb_data` in a named volume |
| `caddy` | TLS-terminating reverse proxy |
| `watchtower` | polls GHCR every 60 s, restarts `app` on a new `latest` |
| `backup` | nightly `tar.gz` of `pb_data` to any S3-compatible bucket, 30-day retention |
| `app-staging` | optional; behind the `staging` compose profile (see below) |

The GitHub workflow (`.github/workflows/build.yml`) runs the Go tests and
the full Playwright suite before every image push — a red suite never
deploys.

Set the image name once: in `deploy/.env` (`APP_IMAGE=ghcr.io/YOU/REPO:latest`).

## Backups

`backup` refuses to start without `BACKUP_S3_BUCKET` — deliberate: a deploy
without backups is a footgun. Restore = stop app, untar into the volume,
start app. `pb_data` contains *everything*: database, uploaded photos,
settings.

If you enable S3 storage for uploads (`s3.enabled` in config.json), photos
live in the bucket instead of `pb_data/storage` — back up both.

## Staging

A second instance of the same compose file, kept out of normal `up`/`pull`
by the `staging` compose profile. It builds the image **on the server** from
a dev checkout (no registry round-trip):

```bash
# on the server
git -C ~/app-dev pull
docker build -t qr-app:staging ~/app-dev
docker compose up -d app-staging
```

Protect it with the basic-auth block in the Caddyfile (webhook path exempt
so payment-provider test webhooks get through) and `X-Robots-Tag: noindex`.
`deploy_staging.sh` automates the loop from your laptop.

## Production checklist

- [ ] `config.json`: `app_url` is the public https URL (it's printed inside
      QR codes — wrong value means broken printed material)
- [ ] `.env`: superuser creds, Lemon Squeezy keys + webhook secret,
      Google OAuth creds, backup bucket
- [ ] Lemon Squeezy webhook points at `https://yourdomain.com/webhook/lemon-squeezy`
- [ ] Google OAuth redirect URI is `https://yourdomain.com/auth/google/callback`
- [ ] PocketBase SMTP configured (admin → Settings → Mail) — password resets
      depend on it
- [ ] Backup container is running and the first nightly archive appeared
- [ ] `/_/` admin reachable, logs enabled (the migration turns them on)

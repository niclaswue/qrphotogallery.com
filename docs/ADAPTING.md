# Adapting the template to a new product

Two kinds of guidance live here:

1. **Per-idea build plans** — how each candidate business maps onto the
   template, what to add, what to delete.
2. **Porting guides** — modules that exist battle-tested in min-pcw
   (PhotoChallenge Wedding) but deliberately stayed out of the template.
   Each section says where the code lives so you can lift it when a product
   needs it.

The golden rule: **the generic model (`events` → `prompts` → `uploads`)
stretches further than you think.** Reach for schema changes only when a
concept truly has no home.

---

## Part 1 — Per-idea build plans

### QR photo challenge for parties & celebrations

This is the template's out-of-the-box product. Work is branding + content:

- Copy: rewrite `data/locales/*.json` for the occasion (birthday, corporate,
  Christmas party…). The starter prompts on the create page are 8 locale
  keys (`create.starters.p1…p8`).
- Consider porting the **prompt idea library** from min-pcw
  (`data/ideas.<lang>.json` + `internal/app/ideas.go` + the category browser
  in `create.js`) once you have >20 curated prompts per occasion — it was a
  meaningful conversion lever there.
- SEO content pages are the growth engine for this category — see the
  content-marketing porting guide below.

### Simple QR photo gallery ("drop your photos here")

The smallest product: one QR, guests upload, everyone can browse.

- **No schema change.** Create every event with exactly one prompt (text
  like "Share your photos!") and `single_qr_mode = true`. The dispatcher
  then always serves that one prompt.
- Simplify the create flow: drop the prompts section from `create.html` /
  `create.js`; `handleCreateSubmit` hard-codes one prompt.
- Guest UX: make `/e/{id}` render the upload form directly instead of the
  "reveal" page — in `handleEventDispatch`, render the `upload` page
  (or redirect to `/e/{id}/{promptID}`).
- The library page becomes the main guest surface; promote the download
  button. Tier ideas: photo count / storage duration / resolution instead
  of prompt count (`Tier.MaxPrompts` → `MaxUploads`; enforce in the upload
  handler where the free-tier single-photo check sits today).
- Delete: prompt editing UI, cards mode + card PDF (keep the poster).

### QR photo bingo with teams

Prompts become bingo squares; teams compete to fill their card.

- **Schema:** add a `teams` collection (`event`, `name`, `color`) in the
  consolidated migration, and a `team` text field on `uploads`.
- Guest identity: on first scan, let the guest pick (or be assigned) a team;
  persist in a cookie next to the existing guest-state cookie — the
  single-QR dispatcher machinery (bitset cookie, `pickPrompt`) already
  handles per-browser state, extend `guestCookie` with a team field.
- Board view: a new page rendering the prompt grid per team with
  filled/empty state — `eventHasUploadSet` already computes
  prompt→has-upload in one query; group by `team` instead.
- Scoring: bingo = rows/columns complete; compute server-side from the
  board. Add a `/e/{id}/board` guest page and show it after each upload
  (change the post-upload redirect).
- Print: the card deck becomes one **bingo sheet per team** — a new Typst
  template (grid of prompts + one QR); `templates/print/_shared/` gives you
  palette + auto-fit text.
- Keep `lock_after_submit` off; bingo wants many uploads per guest.

### QR audio guestbook

Guests leave voice messages instead of photos.

- **Schema:** add a `kind` text field on `uploads` (`"image"`/`"audio"`),
  and widen the file validation: add an audio branch next to
  `detectImageFormat` (sniff for `ID3`/`fLaC`/OGG/`ftypM4A `/WebM-EBML
  magic bytes). Keep the 50 MB cap.
- **Capture UI:** a MediaRecorder page replacing the photo picker:
  `getUserMedia({audio:true})` → `MediaRecorder` (`audio/mp4` on Safari,
  `audio/webm` elsewhere) → attach the Blob to the existing multipart form
  field. This is the one genuinely new frontend piece (~150 lines of JS);
  everything server-side stays the upload handler with a widened format
  check.
- Playback: `<audio controls>` in the library/gallery instead of `<img>`;
  branch on `kind`. The HEIC `display` rendition path is image-only — skip
  it for audio.
- One prompt per event ("Leave us a message!") like the simple gallery, or
  several ("Tell us your favourite memory of the couple", …) — the prompt
  model gives you a themed guestbook for free.
- ZIP export works unchanged (it streams whatever file is stored).
- Product framing: audio guestbooks price higher than photo products —
  revisit tier prices.

### QR live photo slideshow

Photos appear on a projector/TV seconds after upload.

- **New page:** `/e/{id}/slideshow` (owner-only or token-guarded) that
  fullscreens photos. Simplest robust implementation: poll a JSON endpoint
  (`/api/e/{id}/uploads?since=<cursor>`) every ~5s and fade new photos in —
  add both handler and page; no schema change. PocketBase also has a
  realtime subscription API, but the record API is locked superuser-only in
  this template; polling through our own handler keeps the security model
  intact and is plenty for a slideshow.
- Pair with the simple-gallery shape (one prompt, single QR) and show the
  poster QR as an overlay corner on the slideshow so guests can join from
  the screen itself.
- Moderation matters on a big screen: the owner gallery's delete button
  already exists; consider an `approved` bool on uploads plus a small
  moderation queue if the audience is untrusted.

---

## Part 2 — Porting guides (proven in min-pcw)

Each of these exists in production form in the min-pcw repo. File paths
refer to that repo.

### Retargeting / lifecycle email

`internal/app/retarget.go` + `data/emails/retargeting/templates.json` +
`congrats.go` (post-event "how did it go" mail via cron).

- CLI-triggered campaign (`./app retarget [--send]`), dry-run by default;
  selects users by signup state, respects `marketing_opt_out` (field already
  in this template's schema), stamps `retargeting_sent_at`/`_kind` (add
  those fields when porting), renders localized templates, sends over
  PocketBase's configured SMTP.
- The congrats cron shows the pattern for date-triggered lifecycle mail
  (`registerCongratsCron` — a daily scan for events whose date just passed).

### Business-ops toolkit (`agent_ops/`)

A read-only operating toolkit for running the business with cron-launched
agents: per-source data scripts (Lemon Squeezy orders, PostHog HogQL,
Search Console, infra healthcheck, S3 backup fetch), `report.sh` for an
all-sources timespan report, and cron scenarios (`daily_ops` /
`weekly_growth` / `monthly_review`) that write journal entries and can run a
headless `claude -p` analysis. Guardrail: nothing sends, posts, spends or
writes to prod. Pairs with read-only app subcommands (`stats`, `logs`
CLIs in min-pcw's `stats.go` / `logs_cmd.go`) — port those alongside.

### SEO content machinery

min-pcw's growth engine: 8 guide pages + an interactive planner tool, a
widened sitemap, and **guide-only languages** (the top article translated
into 7 extra languages with partial locale bundles falling back to English).
Port when a product is content-led: `handlers_pages.go` guide handlers,
`views/guide_*.html`, the `GuideLangs` machinery in `internal/i18n` (this
template deliberately removed it — diff the two i18n.go files), plus
`llms.txt`/`ai.txt` handlers in `handlers_seo.go`.

### Card & poster designer UIs

This template ships fixed palettes with a radio picker. min-pcw has full
interactive designers with server-rendered live previews (`POST
/api/card-preview` / `/api/poster-preview` compile the real Typst template
to PNG, so preview == print): `views/card_designer.html`,
`views/poster_designer.html`, `poster_design.go` (a design *object* with
palette/typeface pairing/layout/QR treatment/decor, stored as JSON on the
event), `poster_assets.go` (tinted decorative artwork). Port once design
choice becomes a conversion lever; it's ~2k lines across Go/JS/Typst.

### Per-event prompt translations

Premium feature in min-pcw: one event offered in several guest languages,
with per-prompt translation storage, library-backed auto-translation and a
host override editor. Lives in `challenge.go` (translations maps,
`backfillPromptTranslations`), `guest_lang.go` (per-event negotiation +
switcher), `views/translations.html`. Port only if your product's guests are
genuinely multilingual at the *same* event.

### Reviews / testimonial collection

`handlers_review.go` + `views/rate.html`: a standalone post-event feedback
form (no login; recipient email in a `?u=` token), stored in a `reviews`
collection, triaged by a `reviews` CLI. Feeds testimonials for the landing
page.

### Audio/video media kinds

See the audio guestbook plan above — the same `kind` field approach extends
to short video (sniff MP4/WebM, cap duration client-side, skip the rendition
pipeline or transcode a poster frame).

### Google Ads conversion tracking

Removed from this template. min-pcw's `GoogleAdsConfig` (config.go),
consent-mode gtag bootstrap (`views/base.html`), and `google-ads.js`
show the pattern: env-gated IDs, Consent Mode v2 defaults denied,
lead + purchase conversion events. `GOOGLE_ADS.md` there is a full runbook.

---

## Deleting what you don't need

Modules are isolated so removal is mechanical: delete the handler file +
its routes in `app.go` + its view + its locale keys (the parity test lists
orphans). Concretely:

- **No payments:** delete `handlers_payment.go`, `lemon.go`, the two routes,
  `views/pricing.html` + pricing nav/footer links; set one tier in
  config.json with generous limits.
- **No print:** delete `pdf.go`, `pdf_typst.go`, `templates/print/`,
  `cmd/preview-cards`, the print/poster/qr routes — and the typst stage in
  the Dockerfile.
- **No cards mode / no single-QR mode:** hard-code `single_qr_mode` at
  create time and remove the toggle panel from `views/overview.html`.
- **One language only:** set `SupportedLangs = ["en"]`, delete `de.json`
  and the switcher markup in `views/_nav.html`.

# Adapting the gallery

QR Photo Gallery is already the smallest useful product shape: one event, one
QR, one uploader, and one flat gallery. Keep those invariants unless the new
product genuinely needs a different interaction.

## What to keep simple

- `events` are the customer-owned gallery records.
- each new event receives one hidden `prompts` record solely as an upload
  bucket for database compatibility;
- `/e/{id}` remains the canonical public URL;
- uploads are queried and displayed by `event`, not grouped by bucket;
- the poster and bare QR both point to the same URL;
- the public PocketBase record API stays locked, with access enforced in Go
  handlers.

If starting a new product with no existing data, you may remove the hidden
bucket layer by moving the file relationship directly to `events`. That is a
schema cleanup, not a user-facing feature, so weigh it against the simplicity
of retaining the working pipeline.

## Brand or vertical variant

For a wedding-specific, conference-specific, school, sports, or venue version,
the application flow usually does not need to change:

1. update the product name, public URL, support address, and prices in
   `config.json`;
2. rewrite both locale bundles around that audience;
3. replace the landing and social imagery;
4. change the shared CSS tokens;
5. update the fixed poster palette and wording;
6. replace the legal documents and configure payments.

Avoid adding host-selectable looks merely to create a vertical. One strong
brand treatment keeps setup fast and every surface consistent.

## Live slideshow

A slideshow is the closest feature extension because it consumes the same flat
gallery.

- Add an owner-only or signed-token `/e/{id}/slideshow` page.
- Add a small JSON handler that returns uploads newer than a cursor.
- Poll every few seconds and fade new files into a fullscreen view.
- Reuse `galleryMediaItems` and `uploadDisplayURL` so HEIC renditions and video
  classification remain consistent.
- If the audience is untrusted, add moderation before displaying files on a
  public screen.

Polling through an application handler keeps the existing access-control model
and is sufficient for ordinary event traffic.

## Moderated gallery

Add an optional `approved` boolean to uploads and a Commercial dashboard queue.
Public queries should include only approved records; owners continue to see
all records. Keep deletion independent from approval so moderation remains a
reversible action.

For a private gallery, prefer a single event access code or signed share token
over guest accounts. Guests should still reach the uploader in one scan.

## Audio guestbook

The record and ZIP pipeline can also collect voice messages:

- add a media kind and allowlist for MP4 audio, WebM audio, OGG, or FLAC;
- build a MediaRecorder picker that attaches its Blob to the existing multipart
  form;
- render `<audio controls>` in the shared and owner galleries;
- skip the image rendition worker for audio files;
- revisit per-file limits and the product copy.

Keep one QR and one flat feed unless separate questions are an explicit part of
the new product.

## Client and agency workflow

The Commercial tier already separates licensing and client-facing controls
from the complete core gallery. Likely next steps are:

- an optional client logo and neutral footer;
- a custom expiry date within a bounded retention policy;
- a signed client download link;
- an internal gallery label separate from the public event title;
- account-level lists, search, and archive filters for frequent hosts.

These belong to the event/account model and do not require alternate QR modes.

## Content and lifecycle modules

The repository intentionally keeps marketing operations out of the product
runtime. Common additions are localized guide pages, post-event reminder
emails, review collection, and read-only KPI/reporting commands. Add each as an
isolated handler/routes/view/locale-key module so it remains easy to remove.

## Removing modules

- **No payments:** remove the payment handlers, webhook, pricing page, and
  checkout links; give the only configured tier full access.
- **No print:** remove `pdf.go`, `pdf_typst.go`, `templates/print/`, the poster
  route, Typst dependency, and Docker build stage. Keep the QR PNG if hosts
  still need it for their own signs.
- **One language:** keep only the default language metadata/bundle/legal files
  and remove the language switcher.
- **No public ZIP:** remove the guest download route and Commercial toggle;
  leave the authenticated host export intact.

Modules should remain one handler file plus routes, view, assets, and locale
keys wherever possible.

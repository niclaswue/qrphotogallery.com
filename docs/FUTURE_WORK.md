# Future work

Backlog items surfaced during the landing-page redesign that require backend
or content work beyond the marketing pages. Ordered roughly by conversion
impact.

## 1. Frictionless post-payment account creation

**Goal:** let a visitor pay *before* creating an account, and create the
account on their behalf afterwards — removing the biggest drop-off in the
funnel (the register wall between "Create my gallery" and checkout).

**Today:** `handleCreateSubmit` → if unauthenticated, the visitor is pushed
through register/login before the gallery is saved and before checkout
(`handlers_create.go`, `handlers_auth.go`, `handlers_payment.go`). Every extra
field is a place to lose an impulse buyer.

**Proposed flow:**

1. Guest enters a gallery name on `/create` and clicks the CTA. Persist the
   pending gallery (name + date) in a short-lived signed cookie or a
   `pending_events` row — no account yet.
2. Send them straight to Lemon Squeezy checkout. Collect their email at
   checkout (Lemon Squeezy already does this).
3. On the `order_created` webhook (`lemon.go` / `handleLemonWebhook`):
   - Look up or create a `users` record for the checkout email.
   - Generate a random password, mark the account for a forced reset.
   - Materialise the pending gallery against that user and upgrade the tier.
4. Redirect the post-checkout page (`payment_success`) to a "Set your password"
   / magic-login screen. Also email a one-click login link (PocketBase
   `authWithOTP` or a signed login token) so they never type a password.

**Touches:** `handlers_create.go`, `handlers_payment.go`, `lemon.go`,
`handlers_auth.go`, a new migration for `pending_events` (or a cookie), one new
email template, and copy in both locales. Keep the existing "register first"
path working for users who arrive logged-out on `/login`.

**Risks:** duplicate accounts if the checkout email differs from an existing
account — reconcile by email; idempotent webhook handling is mandatory.

## 2. Live example gallery

The landing hero QR (`/static/img/qr-sample.svg`) and the "See it in action"
section promise a *real* gallery guests can open. Today the QR encodes the
homepage. To make the promise literal:

- Seed one permanent demo event with ~15 tasteful, rights-cleared photos and a
  short video, owned by a system account, non-expiring.
- Expose it at a stable slug, e.g. `/e/demo`, and regenerate the hero QR to
  encode `https://qrphotogallery.com/e/demo` (rerun the throwaway generator in
  git history: `go run ./cmd/qrgen "<url>" pb_public/static/img/qr-sample.svg`).
- Point the example section's "scan the code above" copy at it, and consider a
  visible "Open the live gallery →" button.
- Make the demo read-only (or auto-pruning) so visitors can't fill it with
  uploads.

Until then, the landing "example" is an illustrative preview (styled
placeholder tiles), which is honest but not interactive.

## 3. SEO landing pages (footer expansion)

Traffic strategy is SEO, so the footer is built to grow into a hub of
keyword-targeted pages. Right now the "Perfect for" footer links point at the
homepage `#uses` anchor. Replace each with a dedicated, indexable page.

**Planned URLs (one page per intent):**

- `/qr-code-photo-sharing-wedding`
- `/qr-code-photo-sharing-birthday-party`
- `/qr-code-photo-sharing-corporate-events`
- `/qr-code-photo-sharing-family-reunion`
- `/wedding-photo-app-alternative`, `/event-photo-app-alternative` (comparison
  intent — reuse the pricing comparison table)
- Guides: `/how-to-collect-guest-photos-at-a-wedding`, `/qr-code-for-photos-how-to`

**How to add one (keep it boring and deletable):**

1. New handler in `handlers_seo.go` (or a small `handlers_content.go`) that
   renders a shared `content_page` view with per-page locale keys.
2. Register via `registerLocalisedGet` in `app.go` so every language prefix is
   covered.
3. Add the path to `publicSitemapPaths` in `handlers_seo.go` (drives
   `sitemap.xml` + hreflang alternates automatically).
4. Add the copy to **both** `data/locales/en.json` and `de.json` (parity is
   test-enforced) and link it from `_footer.html`.
5. Each page: unique `<h1>`, unique meta description, a clear CTA to `/create`,
   internal links to `/pricing` and 2–3 sibling pages.

Because the ServeMux `/` catch-all renders the landing page for unknown GETs,
only wire footer links to routes that actually exist — add the page first, then
the link.

## 4. Trust signals as customers arrive

The trust band is deliberately honest for a zero-customer launch (guarantee,
free trial, one-time payment, EU hosting) rather than fake logos or review
counts. As real proof accumulates, add — without overclaiming:

- Count of galleries created / photos collected (once meaningful).
- Genuine testimonials with attribution and consent.
- A gallery-count or storage-served ticker fed from real data.

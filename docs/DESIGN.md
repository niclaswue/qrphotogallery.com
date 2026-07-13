# Design system

The product has one coherent visual identity. Hosts do not choose themes and
events do not alter the guest page palette. Rebranding therefore changes the
product once rather than introducing another settings surface.

## Web tokens

Shared tokens are at the top of `pb_public/static/css/main.css`:

```css
:root {
    --color-primary: #e3604d;
    --color-primary-dark: #c94737;
    --color-primary-soft: #fff0ec;
    --color-dark: #11243a;
    --color-text: #24364a;
    --color-text-muted: #68778a;
    --color-bg: #fbfaf7;
    --color-card: #fff;
    --color-section-alt: #f2f5f4;
    --color-border: #dfe5e7;
    --font-display: "Cormorant Garamond", Georgia, serif;
    --font-body: "Jost", sans-serif;
}
```

Changing the primary, dark, background, and muted colors updates buttons,
navigation, forms, cards, gallery chrome, and focus states. Keep sufficient
contrast and update the `theme-color` meta value in `views/base.html` plus the
colors in `pb_public/static/favicon.svg` at the same time.

## Typography

The interface pairs Cormorant Garamond for large editorial headings with Jost
for UI and body copy. WOFF2 files are self-hosted in
`pb_public/static/fonts/`; `@font-face` rules live at the beginning of
`main.css`, and `base.html` preloads the critical files.

To change fonts, replace the WOFF2 assets, update those declarations and the
two font tokens, then inspect landing, create, pricing, dashboard, and guest
gallery pages at desktop and phone widths. Large translated headings are the
most likely place for a new typeface to change layout.

## Stylesheet boundaries

- `main.css` — tokens, reset, navigation, footer, shared buttons/forms,
  language picker, and consent panel
- `landing.css` — landing content and reusable FAQ sections
- `create.css` — the name/date setup page
- `pricing.css` — Personal and Commercial offer comparison
- `app.css` — authenticated event list and unified host dashboard
- `gallery.css` — combined guest uploader, flat media grid, and lightbox
- `auth.css` — login, registration, password reset, and shared errors

Each page template declares only the page sheets it needs. Responsive rules
live with their owning components.

## Product imagery

`pb_public/static/img/hero-gallery.webp` is the primary landing visual and
`og-default.jpg` is the social sharing image. Keep meaningful image dimensions
in the HTML to prevent layout shifts and compress replacements before commit.
The logo mark is CSS/SVG-native and should remain legible at the small mobile
navigation size.

## QR poster

`templates/print/poster.typ` is the only print layout. It receives a fixed
product palette from `posterPalette` in `internal/app/pdf.go`; that palette is
not stored on events or exposed as a setting. The poster uses bundled Fraunces
and Space Grotesk TTF files from `data/fonts/`, independently of the web font
files.

Title text is auto-fitted into a bounded region so long event names cannot
push the QR off the A4 page. Preserve that guarantee when adjusting the
template. Verify changes with:

```bash
go test ./internal/app -run TestRenderTypstPoster
```

## Visual QA checklist

After a meaningful UI change, inspect at least:

- English and German landing/create/pricing pages;
- mobile navigation and all primary CTAs;
- empty and populated guest galleries;
- upload selection, progress, validation, and success states;
- empty and populated host dashboards;
- focus states, lightbox keyboard navigation, and long gallery titles;
- the rendered PDF poster and scannability of its QR code.

# Base design & theming

The template ships one polished, coherent look rather than many mediocre
options. Rebranding is a matter of swapping tokens in three places — the
structure never changes.

## 1. Site chrome — CSS custom properties

All site styling hangs off the token block at the top of the `:root`
section in `pb_public/static/css/main.css` (~line 244):

```css
:root, .theme-classic {
    --color-primary: #C0825E;   /* brand terracotta — buttons, links, accents */
    --color-bg: #FAF8F5;        /* warm off-white page background */
    --color-dark / -text / -muted / -border / -success / -error ...
    --font-display: 'Cormorant Garamond', Georgia, serif;
    --font-body: 'Jost', sans-serif;
    --radius-* / --shadow-*     /* geometry & depth scale */
}
```

Rebrand = change `--color-primary` (+ its light/dark shades), maybe
`--color-bg`, and you're 80 % done. Alternative full palettes
(`.theme-sage`, `.theme-rose`, `.theme-navy`, …) sit right below the
`:root` block — either promote one to `:root` or set the class on `<body>`.

Also update `theme-color` in `views/base.html` (mobile browser chrome) and
`pb_public/static/favicon.svg`.

### Fonts

The pairing is **Cormorant Garamond** (display serif) + **Jost** (body
sans), self-hosted as woff2 subsets in `pb_public/static/fonts/` with
`@font-face` rules at the top of `main.css` and preloads in `base.html`. To
swap: download woff2 subsets (google-webfonts-helper works well), replace
the `@font-face` blocks and the two `--font-*` tokens, update the preloads.

### CSS layout

Per-page stylesheets keep the cascade small: `main.css` (tokens, reset,
nav/footer, buttons, forms) is on every page; `landing.css`, `create.css`,
`pricing.css`, `app.css` (authed dashboard), `challenge.css` (guest pages),
`auth.css` load per page via each view's `head` block.

## 2. Event palettes — shared by web and print

`internal/app/designs.go` defines the five palettes hosts can pick per
event (classic, romantic, boho, modern, garden): five hex values each
(primary/secondary/accent/background/text). They theme:

- the guest-facing pages (inline styles from the `Design` template data),
- the printed card deck and poster (passed to Typst as `printDesign`).

Because both surfaces read the same struct, print material always matches
the guest pages. Add/replace palettes by editing that one slice — the create
form, overview picker, and print pipeline pick changes up automatically.
Keep `classic` as the first entry (it's the fallback for unknown IDs).

## 3. Print — Typst templates

`templates/print/`:

- `classic.typ` — the card deck (A4 sheets, 8 cards each, duplex-mirrored
  backs, crop marks) + single-card preview modes. Fonts: Fraunces (display)
  + Space Grotesk (labels), bundled in `data/fonts/`.
- `poster.typ` — the single-QR poster (A4).
- `_shared/` — sheet layout, palette conversion, and `auto-fit-text` (steps
  the font size down until a block fits its budget — the reason no host
  input can ever overflow a card; preserve this when editing).

Per-design print layouts are supported: `renderTypstCards` looks for
`templates/print/<design-id>.typ` and falls back to `classic.typ` (with the
selected palette still applied). So a new visual direction = one new .typ
file, no Go changes.

Print fonts live in `data/fonts/` (TTF, read by `typst --font-path`); the
woff2 files in `pb_public/static/fonts` are the *web* copies — they're
separate on purpose.

## Design intent

The default look is intentionally "editorial, warm, print-adjacent" — it
photographs well next to real stationery and reads premium at small sizes.
When adapting, decide the mood first (a kids' birthday product and an audio
guestbook for weddings shouldn't share a palette), swap tokens, then check
the three surfaces in one pass: landing page, guest upload page, printed
poster. `go run ./cmd/preview-cards` regenerates card imagery for marketing
pages.

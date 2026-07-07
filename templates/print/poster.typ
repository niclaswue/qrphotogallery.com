// Single-QR poster: one A4 page with a large QR pointing at the event
// dispatcher (app_url/e/{id}). Themed by the event's design palette — the
// same palette the printed cards use, so all print material matches.
//
// data.json carries `design` (hex palette, see app.printDesign) and `poster`
// (final localized texts, see app.posterRender). The QR PNG is pre-generated
// per job at <render>/qr/event.png.
//
// Layout guarantee: the title is auto-fitted (font size steps down until it
// fits its band), so no input can push content off the page. Keep that
// guarantee when editing.

#import "/templates/print/_shared/palette.typ": palette
#import "/templates/print/_shared/typography.typ": auto-fit-text

#let render = sys.inputs.at("render", default: ".")
#let data = json("/" + render + "/data.json")
#let P = data.poster
#let pal = palette(data.design)

#let muted(c) = c.transparentize(45%)

#set page(paper: "a4", margin: 0mm, fill: pal.background)

#let pad-x = 24mm
#let content-w = 210mm - 2 * pad-x

// Top: small uppercase heading with flanking rules.
#place(top + center, dy: 30mm,
  stack(dir: ltr, spacing: 4mm,
    align(horizon, line(length: 14mm, stroke: 0.6pt + pal.text)),
    text(
      font: "Space Grotesk",
      size: 11pt,
      weight: 500,
      tracking: 2.6pt,
      fill: pal.text,
      upper(P.heading),
    ),
    align(horizon, line(length: 14mm, stroke: 0.6pt + pal.text)),
  ))

// Event title: large display serif, auto-fitted into its band.
#place(top + left, dx: pad-x, dy: 44mm,
  box(width: content-w, height: 42mm,
    align(horizon + center,
      auto-fit-text(
        P.title,
        width: content-w,
        height: 42mm,
        max-pt: 44,
        min-pt: 20,
        step: 1,
        font: "Fraunces",
        weight: 300,
        color: pal.text,
        leading: 0.32em,
      ))))

// Center: the QR code with a hairline frame and accent corner ticks.
#let qr-size = 84mm
#let qr-y = 100mm
#place(top + center, dy: qr-y,
  box(width: qr-size, height: qr-size,
    stroke: 0.8pt + pal.text, inset: 6mm, fill: white,
    image("/" + render + "/qr/event.png", width: 100%, height: 100%)))

// "Scan me" label under the QR.
#place(top + center, dy: qr-y + qr-size + 7mm,
  text(
    font: "Fraunces",
    size: 14pt,
    style: "italic",
    fill: pal.accent,
    P.scan_me,
  ))

// Instruction line.
#place(top + left, dx: pad-x, dy: qr-y + qr-size + 20mm,
  box(width: content-w,
    align(center,
      par(leading: 0.5em,
        text(
          font: "Fraunces",
          size: 13pt,
          weight: 400,
          fill: pal.text,
          P.caption,
        )))))

// Footer: date (when set) and the app URL attribution.
#place(bottom + center, dy: -18mm,
  stack(spacing: 3.4mm,
    if data.event_date != "" {
      text(
        font: "Space Grotesk",
        size: 8pt,
        weight: 400,
        tracking: 1.6pt,
        fill: muted(pal.text),
        upper(data.event_date),
      )
    },
    text(
      font: "Space Grotesk",
      size: 8pt,
      weight: 400,
      tracking: 1.6pt,
      fill: muted(pal.text),
      upper(P.footer),
    ),
  ))

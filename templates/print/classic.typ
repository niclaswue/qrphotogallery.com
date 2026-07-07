// Classic White — "Folio" editorial design.
// Modern minimal: cream background, ink black, Fraunces display serif paired
// with Space Grotesk uppercase labels. Front centers a large prompt with one
// word italicised; back is a precise QR + step-tag layout.
//
// Inspired by the Folio sketch from the Wedding Cards design exploration.

#import "/templates/print/_shared/layout.typ" as L
#import "/templates/print/_shared/palette.typ": palette
#import "/templates/print/_shared/typography.typ": auto-fit-text

#let render = sys.inputs.at("render", default: ".")
#let mode = sys.inputs.at("mode", default: "sheet")
#let data = json("/" + render + "/data.json")

// Card padding inspired by Folio's 30px / 32px (≈ 5.5–6 mm at the design scale).
#let pad-x = 5.5mm
#let pad-y = 5mm

// Subtle "muted" variants of theme colors for tertiary text and rules.
#let muted(c) = c.transparentize(55%)
#let hairline(c) = c.transparentize(72%)

// Pad an integer so single-digit prompt numbers read as "07".
#let two-digit(n) = if n < 10 { "0" + str(n) } else { str(n) }

// Roman numerals for the back-side photo index, kept short and serif-y.
#let to-roman(n) = {
  let pairs = (
    (1000, "M"), (900, "CM"), (500, "D"), (400, "CD"),
    (100,  "C"), ( 90, "XC"), ( 50, "L"), ( 40, "XL"),
    ( 10,  "X"), (  9, "IX"), (  5, "V"), (  4, "IV"),
    (  1,  "I"),
  )
  let m = n
  let out = ""
  for pair in pairs {
    while m >= pair.at(0) {
      out = out + pair.at(1)
      m = m - pair.at(0)
    }
  }
  out
}

// Localized labels. Callers may override any key by setting `labels` on the
// data object; missing keys fall back to the English defaults below.
#let default-labels = (
  photo: "Photo",
  how_it_works: "How it works",
  instruction_lead: "Scan, snap, and add it to the album ",
  instruction_emph: "instantly",
  instruction_tail: ".",
  no: "No.",
)
#let label-of(key) = {
  let overrides = data.at("labels", default: (:))
  overrides.at(key, default: default-labels.at(key))
}

#let card(prompt, data, kind) = {
  let p = palette(data.design)

  // Solid background fill
  place(top + left, dx: 0mm, dy: 0mm,
    rect(width: L.card-w, height: L.card-h, fill: p.background, stroke: none))

  if kind == "front" {
    // Header (top): couple name on left, photo index on right
    place(top + left, dx: pad-x, dy: pad-y,
      text(
        font: "Space Grotesk",
        size: 5.2pt,
        weight: 500,
        tracking: 1.25pt,
        fill: p.text,
        upper(data.event_title),
      ))
    place(top + right, dx: -pad-x, dy: pad-y - 0.6mm,
      text(
        font: "Fraunces",
        size: 7pt,
        style: "italic",
        fill: p.text,
        label-of("photo") + " · " + two-digit(prompt.sort_order),
      ))

    // Footer (bottom): short rule on the left, URL on the right
    place(bottom + left, dx: pad-x, dy: -pad-y - 1.4mm,
      line(length: 9mm, stroke: 0.55pt + p.text))
    place(bottom + right, dx: -pad-x, dy: -pad-y,
      text(
        font: "Space Grotesk",
        size: 4.6pt,
        weight: 400,
        tracking: 1.1pt,
        fill: muted(p.text),
        upper(data.app_url_display),
      ))

    // Prompt body: large display serif, bottom-aligned within the middle band.
    // The band sits between the header and the footer.
    let band-top    = pad-y + 6mm
    let band-bottom = L.card-h - pad-y - 7mm
    let band-h      = band-bottom - band-top
    let band-w      = L.card-w - 2 * pad-x

    place(top + left, dx: pad-x, dy: band-top,
      box(width: band-w, height: band-h,
        align(bottom + left,
          auto-fit-text(
            prompt.text,
            width: band-w,
            height: band-h,
            max-pt: 18,
            min-pt: 10,
            step: 0.5,
            font: "Fraunces",
            weight: 300,
            color: p.text,
            leading: 0.28em,
          ))))
  } else {
    // BACK
    let qr-size = 32mm
    let qr-x    = pad-x
    let qr-y    = (L.card-h - qr-size) / 2

    // QR with a 1pt black frame (Folio's hairline border)
    place(top + left, dx: qr-x, dy: qr-y,
      box(width: qr-size, height: qr-size,
        stroke: 0.6pt + p.text, inset: 0pt, radius: 0pt,
        image(
          "/" + render + "/qr/" + prompt.id + ".png",
          width: qr-size,
          height: qr-size,
        )))

    // Right column: step tag, instruction, then a meta row at the bottom.
    let text-x = pad-x + qr-size + 5mm
    let text-w = L.card-w - text-x - pad-x

    // "— HOW IT WORKS" : a 5mm rule + uppercase label
    place(top + left, dx: text-x, dy: pad-y + 5mm,
      stack(dir: ltr, spacing: 2.2mm,
        align(horizon, line(length: 5mm, stroke: 0.55pt + p.text)),
        text(
          font: "Space Grotesk",
          size: 5pt,
          weight: 600,
          tracking: 1.4pt,
          fill: p.text,
          upper(label-of("how_it_works")),
        )))

    // Instruction (Fraunces, with one italic emphasis)
    place(top + left, dx: text-x, dy: pad-y + 12mm,
      box(width: text-w,
        text(
          font: "Fraunces",
          size: 9.5pt,
          weight: 400,
          fill: p.text,
          [#label-of("instruction_lead")] +
          text(style: "italic", weight: 400)[#label-of("instruction_emph")] +
          [#label-of("instruction_tail")],
        )))

    // Tertiary line (small): "No. VII"
    place(top + left, dx: text-x, dy: pad-y + 28mm,
      text(
        font: "Fraunces",
        size: 6.5pt,
        style: "italic",
        fill: muted(p.text),
        label-of("no") + " " + to-roman(prompt.sort_order),
      ))

    // Meta row: hair-thin rule + small uppercase URL
    place(bottom + left, dx: text-x, dy: -pad-y,
      stack(dir: ltr, spacing: 2.2mm,
        align(horizon, line(length: 12mm, stroke: 0.4pt + hairline(p.text))),
        text(
          font: "Space Grotesk",
          size: 4.6pt,
          weight: 400,
          tracking: 1.1pt,
          fill: muted(p.text),
          upper(data.app_url_display),
        )))
  }
}

#if mode == "preview-front" or mode == "preview-back" {
  // Single-card render at exact card dimensions for preview imagery.
  set page(width: L.card-w, height: L.card-h, margin: 0mm)
  let kind = if mode == "preview-front" { "front" } else { "back" }
  card(data.prompts.first(), data, kind)
} else {
  L.render-sheets(card, data)
}

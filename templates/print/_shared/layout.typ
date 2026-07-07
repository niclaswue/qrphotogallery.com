// A4 sheet layout for 91x59mm landscape cards, 2 columns x 4 rows = 8/sheet.
// Front and back grids; back columns are mirrored for long-edge duplex flip.
// Crop marks at all four card corners for guillotine cutting.

#let card-w = 91mm
#let card-h = 59mm
#let cols = 2
#let rows = 4
#let page-margin-x = 14mm   // (210 - 2*91) / 2
#let page-margin-y = 30.5mm // (297 - 4*59) / 2

#let mark-len = 3mm
#let mark-offset = 1mm
#let mark-stroke = 0.2pt + rgb("#888888")

#let card-position(col, row) = (
  x: page-margin-x + col * card-w,
  y: page-margin-y + row * card-h,
)

#let front-slot(idx) = (
  col: calc.rem(idx, cols),
  row: calc.quo(idx, cols),
)

#let back-slot(idx) = (
  col: cols - 1 - calc.rem(idx, cols),
  row: calc.quo(idx, cols),
)

#let crop-marks(col, row) = {
  let pos = card-position(col, row)
  let pts = (
    (pos.x, pos.y),
    (pos.x + card-w, pos.y),
    (pos.x, pos.y + card-h),
    (pos.x + card-w, pos.y + card-h),
  )
  for pt in pts {
    let px = pt.at(0)
    let py = pt.at(1)
    place(top + left, dx: px - mark-len - mark-offset, dy: py,
      line(length: mark-len, stroke: mark-stroke))
    place(top + left, dx: px + mark-offset, dy: py,
      line(length: mark-len, stroke: mark-stroke))
    place(top + left, dx: px, dy: py - mark-len - mark-offset,
      line(length: mark-len, angle: 90deg, stroke: mark-stroke))
    place(top + left, dx: px, dy: py + mark-offset,
      line(length: mark-len, angle: 90deg, stroke: mark-stroke))
  }
}

#let render-sheets(card-fn, data) = {
  set page(paper: "a4", margin: (x: 0mm, y: 0mm))

  let cards-per-sheet = cols * rows
  let total = data.prompts.len()
  let sheets = calc.ceil(total / cards-per-sheet)

  for s in range(sheets) {
    if s > 0 { pagebreak() }
    let start = s * cards-per-sheet
    let end = calc.min(start + cards-per-sheet, total)
    let chunk = data.prompts.slice(start, end)

    // FRONT
    for i in range(chunk.len()) {
      let prompt = chunk.at(i)
      let slot = front-slot(i)
      let pos = card-position(slot.col, slot.row)
      crop-marks(slot.col, slot.row)
      place(top + left, dx: pos.x, dy: pos.y,
        box(width: card-w, height: card-h, clip: true,
          card-fn(prompt, data, "front")))
    }

    pagebreak()

    // BACK (mirrored columns for long-edge duplex)
    for i in range(chunk.len()) {
      let prompt = chunk.at(i)
      let slot = back-slot(i)
      let pos = card-position(slot.col, slot.row)
      crop-marks(slot.col, slot.row)
      place(top + left, dx: pos.x, dy: pos.y,
        box(width: card-w, height: card-h, clip: true,
          card-fn(prompt, data, "back")))
    }
  }
}

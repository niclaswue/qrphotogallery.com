// Auto-fit body text into a box of (width, height) by trying decreasing
// font sizes from max-pt down to min-pt in `step` increments. Returns a
// boxed paragraph at the largest size that fits.

#let auto-fit-text(
  body,
  width: 80mm,
  height: 30mm,
  max-pt: 13,
  min-pt: 8.5,
  step: 0.5,
  font: "EB Garamond",
  weight: 400,
  color: black,
  leading: 0.55em,
) = context {
  let chosen = min-pt
  let trial = max-pt
  while trial >= min-pt {
    let probe = box(
      width: width,
      par(
        leading: leading,
        text(size: trial * 1pt, font: font, weight: weight, fill: color, body),
      ),
    )
    let m = measure(probe)
    if m.height <= height {
      chosen = trial
      break
    }
    trial = trial - step
  }
  box(
    width: width,
    par(
      leading: leading,
      text(size: chosen * 1pt, font: font, weight: weight, fill: color, body),
    ),
  )
}

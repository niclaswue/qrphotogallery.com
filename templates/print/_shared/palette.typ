// Convert the fixed product palette from data.json into Typst color objects.
// `value` is a dict with hex string fields: primary, secondary, accent,
// background, text.

#let palette(value) = (
  primary:    rgb(value.primary),
  secondary:  rgb(value.secondary),
  accent:     rgb(value.accent),
  background: rgb(value.background),
  text:       rgb(value.text),
)

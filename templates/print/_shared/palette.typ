// Convert the design colors from data.json into typst color objects.
// `design` is a dict with hex string fields: primary, secondary, accent,
// background, text.

#let palette(design) = (
  primary:    rgb(design.primary),
  secondary:  rgb(design.secondary),
  accent:     rgb(design.accent),
  background: rgb(design.background),
  text:       rgb(design.text),
)

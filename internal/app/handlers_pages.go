package app

import (
	"fmt"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

func handleHome(e *core.RequestEvent) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	stdCents, premCents := tierPriceCents(appConfig.Tiers)
	return e.HTML(http.StatusOK, renderWithBase(e, "landing", map[string]any{
		"StandardPriceEUR": stdCents / 100,
		"PremiumPriceEUR":  premCents / 100,
		"StandardPrice":    formatTierPrice(stdCents, lang),
	}))
}

// tierPriceCents returns the standard and premium price in cents from the
// given tier set. Used to keep schema.org Offer values in sync with config.
func tierPriceCents(tiers []TierConfig) (standard, premium int) {
	for _, t := range tiers {
		switch t.Name {
		case "standard":
			standard = t.PriceCents
		case "premium":
			premium = t.PriceCents
		}
	}
	return
}

func handlePricing(e *core.RequestEvent) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	_, stdPrice, premPrice := tierPrices(appConfig.Tiers, lang)

	return e.HTML(http.StatusOK, renderWithBase(e, "pricing", map[string]any{
		"StandardPrice": stdPrice,
		"PremiumPrice":  premPrice,
		"SupportEmail":  appConfig.SupportEmail,
	}))
}

func handleLegal(e *core.RequestEvent) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	sections := legalSections[lang]
	return e.HTML(http.StatusOK, renderWithBase(e, "legal", map[string]any{
		"Imprint": sections["imprint"],
		"Privacy": sections["privacy"],
		"Refund":  sections["refund"],
	}))
}

// tierPrices returns locale-formatted display prices for free/standard/
// premium from the given tier set. Driven by PriceCents so non-English
// locales keep the trailing-symbol convention ("49 €") while the English
// form stays "€49". The tier's Price field is kept in config.json as
// human-readable documentation only.
func tierPrices(tiers []TierConfig, lang string) (free, standard, premium string) {
	for _, t := range tiers {
		switch t.Name {
		case "free":
			free = formatTierPrice(t.PriceCents, lang)
		case "standard":
			standard = formatTierPrice(t.PriceCents, lang)
		case "premium":
			premium = formatTierPrice(t.PriceCents, lang)
		}
	}
	return
}

// formatTierPrice renders an integer cent amount as a price string in the
// store's currency (EUR). Symbol placement follows locale convention:
// leading "€" for English, trailing " €" for the other supported languages.
func formatTierPrice(cents int, lang string) string {
	eur := cents / 100
	if lang == "en" {
		return fmt.Sprintf("€%d", eur)
	}
	return fmt.Sprintf("%d €", eur)
}

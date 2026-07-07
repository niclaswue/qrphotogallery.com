package app

import (
	"fmt"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

func handleHome(e *core.RequestEvent) error {
	free, _, _ := tierLimits(appConfig.Tiers)
	stdCents, premCents := tierPriceCents(appConfig.Tiers)
	return e.HTML(http.StatusOK, renderWithBase(e, "landing", map[string]any{
		"FreePromptLimit":  free,
		"StandardPriceEUR": stdCents / 100,
		"PremiumPriceEUR":  premCents / 100,
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

// variantPrices is the price string set for one PostHog-flag-toggled
// variant. The pricing page renders these into data attributes so a small
// JS shim can swap the visible price (and the checkout link's ?variant=...)
// when the matching PostHog feature flag is enabled.
type variantPrices struct {
	Name     string
	Standard string
	Premium  string
}

func handlePricing(e *core.RequestEvent) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	tiers := appConfig.Tiers
	free, std, prem := tierLimits(tiers)
	freePrice, stdPrice, premPrice := tierPrices(tiers, lang)

	variants := make([]variantPrices, 0, len(appConfig.PricingVariants))
	for _, v := range appConfig.PricingVariants {
		_, vStd, vPrem := tierPrices(v.Tiers, lang)
		variants = append(variants, variantPrices{Name: v.Name, Standard: vStd, Premium: vPrem})
	}

	return e.HTML(http.StatusOK, renderWithBase(e, "pricing", map[string]any{
		"FreePromptLimit":     free,
		"StandardPromptLimit": std,
		"PremiumPromptLimit":  prem,
		"FreePrice":           freePrice,
		"StandardPrice":       stdPrice,
		"PremiumPrice":        premPrice,
		"PricingVariants":     variants,
		"SupportEmail":        appConfig.SupportEmail,
	}))
}

func handleLegal(e *core.RequestEvent) error {
	return e.HTML(http.StatusOK, renderWithBase(e, "legal", map[string]any{
		"Imprint": legalSections["imprint"],
		"Privacy": legalSections["privacy"],
		"Refund":  legalSections["refund"],
	}))
}

// tierLimits returns the free/standard/premium MaxPrompts from the given
// tier set. Used by the pricing page and landing FAQ so copy stays in sync
// with the real tier limits.
func tierLimits(tiers []TierConfig) (free, standard, premium int) {
	for _, t := range tiers {
		switch t.Name {
		case "free":
			free = t.MaxPrompts
		case "standard":
			standard = t.MaxPrompts
		case "premium":
			premium = t.MaxPrompts
		}
	}
	return
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

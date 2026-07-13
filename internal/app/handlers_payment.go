package app

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// handlePayment redirects the user into a Lemon Squeezy hosted checkout for
// the chosen tier. The user_id is passed via custom_data so the webhook can
// upgrade the right account once payment clears.
func handlePayment(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	plan := e.Request.URL.Query().Get("plan")
	// The pricing page passes ?variant=<name> when a PostHog feature flag
	// has swapped the visitor into an alternate price set, so the LS
	// checkout URL points at the variant's product. Empty or unknown
	// variants fall back to the default tier set.
	variant := e.Request.URL.Query().Get("variant")
	tiers := appConfig.pricingTiers(variant)
	var tier *TierConfig
	for i := range tiers {
		if tiers[i].Name == plan {
			tier = &tiers[i]
			break
		}
	}
	if tier == nil || tier.PriceCents == 0 {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_plan", "error.message.invalid_plan")
	}

	if tier.LemonSqueezyVariantID == "" || appConfig.LemonSqueezy.APIKey == "" {
		return renderHTMLErrorKeys(e, http.StatusServiceUnavailable, "error.title.checkout_unavailable", "error.message.checkout_unavailable")
	}

	redirectURL := strings.TrimRight(appConfig.AppURL, "/") + "/payment/success?plan=" + plan
	checkoutURL, err := createLemonCheckout(tier.LemonSqueezyVariantID, e.Auth.Id, e.Auth.Email(), tier.Name, redirectURL)
	if err != nil {
		return e.InternalServerError("Failed to create checkout", err)
	}
	return e.Redirect(http.StatusSeeOther, checkoutURL)
}

// handlePaymentSuccess is a confirmation page; the webhook is the source of
// truth for tier upgrades, so we re-fetch the user to detect whether the
// upgrade has propagated. The page is reachable without auth because users
// arrive via a Lemon Squeezy redirect that may not carry the session cookie.
func handlePaymentSuccess(e *core.RequestEvent) error {
	purchasedPlan := e.Request.URL.Query().Get("plan")

	currentPlan := ""
	upgradePending := purchasedPlan != ""
	if e.Auth != nil {
		if fresh, err := e.App.FindRecordById("users", e.Auth.Id); err == nil {
			current := getUserTier(fresh)
			currentPlan = current.Name
			upgradePending = purchasedPlan != "" && purchasedPlan != current.Name
		}
	}

	return e.HTML(http.StatusOK, renderWithBase(e, "payment_success", map[string]any{
		"CurrentPlan":    currentPlan,
		"PurchasedPlan":  purchasedPlan,
		"UpgradePending": upgradePending,
	}))
}

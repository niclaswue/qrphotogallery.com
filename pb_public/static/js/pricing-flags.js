// Pricing-page price swap driven by PostHog feature flags.
//
// Default render is the production price (config.json: tiers). When the
// "pricing-higher" flag is enabled the higher-variant price is shown and
// every .price-card checkout link gets ?variant=higher appended so the
// payment handler resolves the matching Lemon Squeezy product. Same for
// "pricing-lower". With both flags off (or PostHog unavailable) nothing
// changes — the default rendering wins.
//
// "pricing-discount-was-price" is a separate flag that reveals the
// strike-through "was €X" markup and a "Save X%" savings pill on the
// paid cards without changing the displayed price. This is an evergreen
// price anchor (not a time-limited launch promo) — the operator can
// leave the flag on permanently. The percentage in the pill is computed
// per-card from the "was" vs current amount, so it always matches what's
// shown. The reveal also requires the "was" amount to be strictly
// greater than the current amount, which keeps the markup honest when a
// pricing variant (higher/lower) bumps the price up or down.
//
// Why client-side: the flags are operator toggles, not visitor cohorts.
// Showing the default price first and swapping on flag load means a flag
// disable takes effect immediately on the next page load, no server
// deploy or cache flush required.
(function () {
    var cards = document.querySelectorAll(".price-card[data-plan]");
    if (!cards.length) return;

    var HIGHER_FLAG = "pricing-higher";
    var LOWER_FLAG = "pricing-lower";
    var DISCOUNT_FLAG = "pricing-discount-was-price";

    function pickVariant() {
        var ph = window.posthog;
        if (!ph || typeof ph.isFeatureEnabled !== "function") return "";
        try {
            if (ph.isFeatureEnabled(HIGHER_FLAG)) return "higher";
            if (ph.isFeatureEnabled(LOWER_FLAG)) return "lower";
        } catch (e) {
            return "";
        }
        return "";
    }

    function applyVariant(variant) {
        cards.forEach(function (card) {
            var priceEl = card.querySelector("[data-price-target]");
            if (priceEl) {
                var attr = variant ? "data-price-" + variant : "data-price-default";
                var next = card.getAttribute(attr);
                if (!next) next = card.getAttribute("data-price-default");
                if (next) priceEl.textContent = next;
            }
            var link = card.querySelector("[data-checkout-link]");
            if (link) {
                var href = link.getAttribute("data-original-href");
                if (!href) {
                    href = link.getAttribute("href") || "";
                    link.setAttribute("data-original-href", href);
                }
                if (variant) {
                    var sep = href.indexOf("?") === -1 ? "?" : "&";
                    link.setAttribute("href", href + sep + "variant=" + encodeURIComponent(variant));
                } else {
                    link.setAttribute("href", href);
                }
            }
        });
        if (window.posthog && typeof window.posthog.register === "function") {
            try {
                window.posthog.register({ pricing_variant: variant || "default" });
            } catch (e) { /* ignore */ }
        }
    }

    // priceNumber extracts the numeric amount from a localised price string,
    // tolerating both "€29" (en) and "29 €" (de/fr/es/it) and the rare
    // "1.299,00" comma decimal — it strips symbols, then normalises decimals.
    function priceNumber(s) {
        if (!s) return 0;
        var raw = String(s).replace(/[^0-9.,-]/g, "");
        if (raw.indexOf(",") !== -1 && raw.indexOf(".") === -1) raw = raw.replace(",", ".");
        else raw = raw.replace(/,/g, "");
        var n = parseFloat(raw);
        return isNaN(n) ? 0 : n;
    }

    function applyDiscountFlag(showDiscount) {
        var revealedAny = false;
        cards.forEach(function (card) {
            var was = card.querySelector("[data-was-price]");
            var pill = card.querySelector("[data-discount-pill]");
            var priceEl = card.querySelector("[data-price-target]");

            // Per-card check: only reveal when the strike-through amount is
            // strictly greater than what's currently shown. If a variant
            // (higher/lower) has pushed the price up to the same level, the
            // strike-through would be redundant or misleading.
            var wasAmount = was ? priceNumber(was.textContent) : 0;
            var nowAmount = priceEl ? priceNumber(priceEl.textContent) : 0;
            var reveal = showDiscount && wasAmount > 0 && nowAmount > 0 && wasAmount > nowAmount;

            if (was) {
                if (reveal) was.removeAttribute("hidden");
                else was.setAttribute("hidden", "");
            }
            if (pill) {
                if (reveal) {
                    // The pill's initial text is a localised template carrying
                    // "%s" (e.g. "Save %s"). Cache it once, then fill in the
                    // computed percentage. Re-evaluations reuse the cached
                    // template so we don't substitute into already-filled text.
                    var tmpl = pill.getAttribute("data-pill-tmpl");
                    if (tmpl === null) {
                        tmpl = pill.textContent;
                        pill.setAttribute("data-pill-tmpl", tmpl);
                    }
                    var pct = Math.round(((wasAmount - nowAmount) / wasAmount) * 100);
                    pill.textContent = tmpl.indexOf("%s") !== -1
                        ? tmpl.replace("%s", pct + "%")
                        : tmpl;
                    pill.removeAttribute("hidden");
                } else {
                    pill.setAttribute("hidden", "");
                }
            }
            if (reveal) revealedAny = true;
        });
        if (window.posthog && typeof window.posthog.register === "function") {
            try {
                window.posthog.register({ pricing_discount_shown: revealedAny });
            } catch (e) { /* ignore */ }
        }
    }

    function evaluate() {
        var variant = pickVariant();
        applyVariant(variant);

        var ph = window.posthog;
        var discount = false;
        if (ph && typeof ph.isFeatureEnabled === "function") {
            try { discount = !!ph.isFeatureEnabled(DISCOUNT_FLAG); } catch (e) { discount = false; }
        }
        applyDiscountFlag(discount);
    }

    document.addEventListener("pcw:posthog-ready", evaluate);
    document.addEventListener("pcw:feature-flags", evaluate);
    // If posthog already loaded before this script attached, the events
    // above already fired; run once now so the swap still happens.
    evaluate();
})();

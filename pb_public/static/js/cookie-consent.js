// Cookie / analytics consent banner.
//
// PostHog is loaded for every visitor so we can measure traffic, but it
// runs in cookieless "memory" persistence until the user accepts. That
// satisfies most consent regimes (no client-side identifier is stored)
// while still giving us pageviews, autocapture, and feature flags.
//
// Storage:
//   localStorage.pcw_consent = "accepted" | "declined"
// On "accepted" we reconfigure PostHog to use localStorage+cookie and
// enable session recording. "declined" keeps the cookieless mode, so we
// still count the visit but never write a persistent identifier.
(function () {
    var KEY = "pcw_consent";
    var hasConfig = !!(window.__appPosthog && window.__appPosthog.key);

    function getChoice() {
        try { return localStorage.getItem(KEY); } catch (e) { return null; }
    }

    function setChoice(v) {
        try { localStorage.setItem(KEY, v); } catch (e) { /* private mode */ }
    }

    // Google Ads Consent Mode v2: granted only after the visitor accepts.
    // base.html sets the default to denied, so this just upgrades. No-op when
    // the gtag tag isn't configured on this deployment.
    function updateAdsConsent(consent) {
        if (consent !== "accepted") return;
        if (typeof window.gtag !== "function") return;
        window.gtag("consent", "update", {
            ad_storage: "granted",
            ad_user_data: "granted",
            ad_personalization: "granted",
            analytics_storage: "granted"
        });
    }

    function loadPostHog(consent) {
        if (!hasConfig) return;
        if (window.__appPosthogLoaded) {
            // Already loaded — just upgrade persistence if consent was granted.
            if (consent === "accepted" && window.posthog && window.posthog.set_config) {
                window.posthog.set_config({
                    persistence: "localStorage+cookie",
                    disable_session_recording: false
                });
                if (window.posthog.startSessionRecording) {
                    try { window.posthog.startSessionRecording(); } catch (e) {}
                }
            }
            return;
        }
        window.__appPosthogLoaded = true;
        window.__appPosthogConsent = consent || "pending";
        var s = document.createElement("script");
        s.src = "/static/js/posthog-init.js";
        s.defer = true;
        document.head.appendChild(s);
    }

    function build() {
        var wrap = document.createElement("aside");
        wrap.className = "cookie-consent";
        wrap.setAttribute("role", "dialog");
        wrap.setAttribute("aria-live", "polite");
        wrap.setAttribute("aria-label", "Cookie consent");
        wrap.innerHTML =
            '<h2>Analytics cookies</h2>' +
            '<p>We count visits anonymously by default. Accept to also enable session replay and remember your visit across pages, which helps us improve the product. ' +
            '<a href="/legal#privacy">Privacy policy</a>.</p>' +
            '<div class="cookie-consent-actions">' +
                '<button type="button" class="btn btn-secondary" data-consent="declined">Decline</button>' +
                '<button type="button" class="btn btn-primary" data-consent="accepted">Accept</button>' +
            '</div>';
        wrap.addEventListener("click", function (ev) {
            var btn = ev.target.closest("[data-consent]");
            if (!btn) return;
            var choice = btn.getAttribute("data-consent");
            setChoice(choice);
            wrap.remove();
            updateAdsConsent(choice);
            loadPostHog(choice);
        });
        return wrap;
    }

    function show() {
        if (!document.body) {
            document.addEventListener("DOMContentLoaded", show);
            return;
        }
        document.body.appendChild(build());
    }

    var choice = getChoice();
    // Returning visitors who already accepted: re-grant ads consent for this
    // pageview (the base.html default starts denied on every load).
    updateAdsConsent(choice);
    // Always start PostHog. If consent isn't decided yet we run cookieless
    // (memory persistence) so we still record the pageview; the banner asks
    // the user to upgrade to persistent tracking. Delay the injection until
    // the page is idle so PostHog doesn't compete with LCP paint.
    function scheduleIdle(fn) {
        if (window.requestIdleCallback) {
            window.requestIdleCallback(fn, { timeout: 2000 });
        } else {
            setTimeout(fn, 1);
        }
    }
    function startPostHog() { loadPostHog(choice || "pending"); }
    if (document.readyState === "complete") {
        scheduleIdle(startPostHog);
    } else {
        window.addEventListener("load", function () { scheduleIdle(startPostHog); }, { once: true });
    }
    if (choice !== "accepted" && choice !== "declined" && hasConfig) {
        show();
    }
})();

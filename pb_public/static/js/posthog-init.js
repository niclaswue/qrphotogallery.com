// PostHog initialisation. Config is injected on the page as window.__appPosthog
// by views/base.html, and consent state ("accepted" | "declined" | "pending")
// is set on window.__appPosthogConsent by cookie-consent.js before this script
// is appended to the page. This file is responsible for:
//   * loading posthog-js with autocapture + (consented) session replay + pageviews
//   * running cookieless (memory persistence) when consent is pending/declined,
//     so we still count traffic without writing a client-side identifier
//   * identifying the current user (and tagging $set props like tier)
//   * firing explicit "outcome" events from <meta name="ph-event"> tags so
//     server-rendered success pages (signup, login, payment) report once
//   * tracking form submits + button clicks tagged with data-ph-event
//   * dispatching a `pcw:posthog-ready` event so feature-flag-driven UI
//     (e.g. the SUMMER26 promo banner) can react when flags are loaded
(function () {
    var cfg = window.__appPosthog;
    if (!cfg || !cfg.key) return;

    var consent = window.__appPosthogConsent || "pending";
    var consented = consent === "accepted";

    !function (t, e) {
        var o, n, p, r;
        e.__SV || (window.posthog = e, e._i = [], e.init = function (i, s, a) {
            function g(t, e) {
                var o = e.split(".");
                2 == o.length && (t = t[o[0]], e = o[1]), t[e] = function () { t.push([e].concat(Array.prototype.slice.call(arguments, 0))) }
            }
            (p = t.createElement("script")).type = "text/javascript", p.crossOrigin = "anonymous", p.async = !0,
                p.src = s.api_host.replace(".i.posthog.com", "-assets.i.posthog.com") + "/static/array.js",
                (r = t.getElementsByTagName("script")[0]).parentNode.insertBefore(p, r);
            var u = e;
            for (void 0 !== a ? u = e[a] = [] : a = "posthog", u.people = u.people || [], u.toString = function (t) {
                var e = "posthog";
                return "posthog" !== a && (e += "." + a), t || (e += " (stub)"), e
            }, u.people.toString = function () { return u.toString(1) + ".people (stub)" }, o = "init capture register register_once register_for_session unregister unregister_for_session getFeatureFlag getFeatureFlagPayload isFeatureEnabled reloadFeatureFlags updateEarlyAccessFeatureEnrollment getEarlyAccessFeatures on onFeatureFlags onSessionId getSurveys getActiveMatchingSurveys renderSurvey canRenderSurvey identify setPersonProperties group resetGroups setPersonPropertiesForFlags resetPersonPropertiesForFlags setGroupPropertiesForFlags resetGroupPropertiesForFlags reset opt_in_capturing opt_out_capturing has_opted_in_capturing has_opted_out_capturing clear_opt_in_out_capturing startSessionRecording stopSessionRecording sessionRecordingStarted captureException loadToolbar get_property getSessionProperty createPersonProfile opt_in_session_recording opt_out_session_recording has_opted_in_session_recording has_opted_out_session_recording clear_opt_in_out_session_recording".split(" "), n = 0; n < o.length; n++) g(u, o[n]);
            e._i.push([i, s, a])
        }, e.__SV = 1)
    }(document, window.posthog || []);

    posthog.init(cfg.key, {
        api_host: cfg.host || "https://eu.i.posthog.com",
        // "always" so we still create person profiles for anonymous visitors,
        // which is what makes them visible on the web traffic dashboard.
        person_profiles: "always",
        persistence: consented ? "localStorage+cookie" : "memory",
        capture_pageview: true,
        capture_pageleave: true,
        autocapture: true,
        disable_session_recording: !consented,
        session_recording: {
            maskAllInputs: true,
            maskInputOptions: { password: true, email: false }
        },
        bootstrap: {
            // Pull featureFlags from the server-rendered config if present, so
            // flag-controlled UI can paint without an extra round-trip.
            featureFlags: (cfg.featureFlags && typeof cfg.featureFlags === "object") ? cfg.featureFlags : undefined
        },
        loaded: function (ph) {
            if (consented && cfg.userId) {
                ph.identify(cfg.userId, {
                    email: cfg.userEmail || undefined,
                    tier: cfg.userTier || undefined
                });
            }
            // Outcome events rendered into the page (e.g. payment_success).
            document.querySelectorAll('meta[name="ph-event"]').forEach(function (m) {
                var name = m.getAttribute("content");
                if (!name) return;
                var props = {};
                Array.from(m.attributes).forEach(function (a) {
                    if (a.name.indexOf("data-") === 0) {
                        props[a.name.slice(5)] = a.value;
                    }
                });
                ph.capture(name, props);
            });
            try {
                document.dispatchEvent(new CustomEvent("pcw:posthog-ready", { detail: { consented: consented } }));
            } catch (e) { /* old browsers */ }
        }
    });

    // When feature flags load (or change), notify listeners so flag-gated UI
    // can react. Used by the pricing-page promo banner.
    posthog.onFeatureFlags(function () {
        try {
            document.dispatchEvent(new CustomEvent("pcw:feature-flags"));
        } catch (e) {}
    });

    // Manual events on tagged elements (forms + buttons + links).
    document.addEventListener("submit", function (ev) {
        var f = ev.target.closest("form[data-ph-event]");
        if (!f || !window.posthog) return;
        window.posthog.capture(f.getAttribute("data-ph-event"), readPhProps(f));
    }, true);

    document.addEventListener("click", function (ev) {
        var el = ev.target.closest("[data-ph-event]");
        if (!el || el.tagName === "FORM") return;
        if (!window.posthog) return;
        window.posthog.capture(el.getAttribute("data-ph-event"), readPhProps(el));
    }, true);

    // Reset identity on logout so the next anonymous session is separate.
    document.addEventListener("click", function (ev) {
        var a = ev.target.closest("a");
        if (!a || !window.posthog) return;
        if (/\/logout(\b|\/|$)/.test(a.getAttribute("href") || "")) {
            window.posthog.capture("logout");
            window.posthog.reset();
        }
    }, true);

    function readPhProps(el) {
        var props = {};
        Array.from(el.attributes).forEach(function (a) {
            if (a.name.indexOf("data-ph-prop-") === 0) {
                props[a.name.slice("data-ph-prop-".length)] = a.value;
            }
        });
        return props;
    }
})();

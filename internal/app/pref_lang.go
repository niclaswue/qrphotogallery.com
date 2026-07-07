package app

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// prefLangCookieName is the site-wide cookie that remembers the language a
// visitor picked from the navbar language switcher, so the choice survives
// across sessions and prefix-less entry points (typing the domain, a
// bookmark, an inbound link). It is independent of the per-event
// language resolution on guest pages, which is derived from the event.
const prefLangCookieName = "pref_lang"

// prefLangMaxAge keeps the preference for a year — long enough that a repeat
// visitor never has to re-pick, short enough to expire eventually.
const prefLangMaxAge = 60 * 60 * 24 * 365

// setPrefLangCookie persists the visitor's chosen site language. Secure
// mirrors the auth cookie so it's never sent over plaintext HTTP in
// production while local dev (http://) still works.
func setPrefLangCookie(e *core.RequestEvent, lang string) {
	e.SetCookie(&http.Cookie{
		Name:     prefLangCookieName,
		Value:    lang,
		Path:     "/",
		MaxAge:   prefLangMaxAge,
		HttpOnly: true,
		Secure:   strings.HasPrefix(appConfig.AppURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
}

// langPrefRedirectPaths are the language-less page paths (matched by prefix,
// except "/" which is matched exactly) where a stored language preference is
// honored with a redirect when the visitor lands on the bare,
// default-language URL. Deliberately limited to the public marketing/funnel
// pages: guest event pages (/e/...) resolve their language from the event,
// and non-HTML routes (assets, downloads, webhooks, the OAuth callback,
// sitemap) have no localized variant to redirect to.
var langPrefRedirectPaths = []string{
	"/pricing", "/legal",
	"/create", "/login", "/register", "/forgot-password", "/overview",
}

// eligibleForLangPrefRedirect reports whether a bare, default-language path is
// one of the public pages where a remembered preference should trigger a
// redirect to the visitor's language.
func eligibleForLangPrefRedirect(langlessPath string) bool {
	if langlessPath == "/" {
		return true
	}
	for _, p := range langPrefRedirectPaths {
		if langlessPath == p || strings.HasPrefix(langlessPath, p+"/") {
			return true
		}
	}
	return false
}

// langPreferenceAction decides what the language-preference middleware should
// do for a request, without touching the response (so it can be unit tested).
// prefCookie is the current pref_lang value (may be empty/stale).
//
//   - setCookie: the language to persist in pref_lang ("" = leave as is)
//   - redirect:  the URL to 302 to ("" = continue to the handler)
//
// Two cases produce action:
//
//  1. An explicit pick from the switcher (?setlang=<lang>): persist the choice
//     and redirect to the clean canonical URL for that language, dropping the
//     marker query but keeping any other params.
//  2. A bare, default-language public page when a non-default preference is
//     stored: redirect to the preferred language so the visitor doesn't have
//     to switch again. Crawlers don't send cookies, so they always see the
//     canonical English page.
func langPreferenceAction(req *http.Request, prefCookie string) (setCookie, redirect string) {
	if req.Method != http.MethodGet {
		return "", ""
	}

	query := req.URL.Query()
	if set := query.Get("setlang"); set != "" {
		if !i18n.IsSupported(set) {
			return "", ""
		}
		_, langlessPath := i18n.FromPath(req.URL.Path)
		query.Del("setlang")
		target := i18n.LangPath(set) + langlessPath
		if rest := query.Encode(); rest != "" {
			target += "?" + rest
		}
		return set, target
	}

	urlLang, langlessPath := i18n.FromPath(req.URL.Path)
	if urlLang != i18n.DefaultLang || !eligibleForLangPrefRedirect(langlessPath) {
		return "", ""
	}
	if i18n.IsSupported(prefCookie) && prefCookie != i18n.DefaultLang {
		target := i18n.LangPath(prefCookie) + langlessPath
		if req.URL.RawQuery != "" {
			target += "?" + req.URL.RawQuery
		}
		return "", target
	}
	return "", ""
}

// applyLangPreference is router middleware that makes the navbar language
// switcher sticky: it persists explicit picks and honors a stored preference
// on later bare-URL visits. The redirect is a temporary 302 so it never
// displaces the canonical English URL in search results.
func applyLangPreference(e *core.RequestEvent) error {
	pref := ""
	if c, err := e.Request.Cookie(prefLangCookieName); err == nil {
		pref = c.Value
	}
	setCookie, redirect := langPreferenceAction(e.Request, pref)
	if setCookie != "" {
		setPrefLangCookie(e, setCookie)
	}
	if redirect != "" {
		return e.Redirect(http.StatusFound, redirect)
	}
	return e.Next()
}

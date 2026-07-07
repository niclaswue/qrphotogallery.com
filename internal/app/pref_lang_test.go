package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLangPreferenceAction locks in the two jobs of the language-preference
// middleware: persisting an explicit switcher pick (?setlang=) and honoring a
// stored preference on a later bare-URL visit — while leaving guest pages,
// non-HTML routes, already-prefixed URLs and crawlers (no cookie) alone.
func TestLangPreferenceAction(t *testing.T) {
	cases := []struct {
		name        string
		method      string
		target      string
		prefCookie  string
		wantSet     string
		wantRedirect string
	}{
		// --- explicit pick from the switcher ---
		{
			name:         "setlang on bare path persists and localizes",
			target:       "/pricing?setlang=de",
			wantSet:      "de",
			wantRedirect: "/de/pricing",
		},
		{
			name:         "setlang back to default clears to english",
			target:       "/de/pricing?setlang=en",
			wantSet:      "en",
			wantRedirect: "/pricing",
		},
		{
			name:         "setlang keeps other query params",
			target:       "/pricing?setlang=de&variant=higher",
			wantSet:      "de",
			wantRedirect: "/de/pricing?variant=higher",
		},
		{
			name:         "setlang on home",
			target:       "/?setlang=de",
			wantSet:      "de",
			wantRedirect: "/de/",
		},
		{
			name:   "setlang with an unsupported language is ignored",
			target: "/pricing?setlang=xx",
		},

		// --- honoring a stored preference ---
		{
			name:         "stored preference redirects bare page",
			target:       "/pricing",
			prefCookie:   "de",
			wantRedirect: "/de/pricing",
		},
		{
			name:         "stored preference redirects home",
			target:       "/",
			prefCookie:   "de",
			wantRedirect: "/de/",
		},
		{
			name:         "stored preference keeps existing query",
			target:       "/pricing?variant=higher",
			prefCookie:   "de",
			wantRedirect: "/de/pricing?variant=higher",
		},

		// --- cases that must NOT redirect ---
		{
			name:       "already prefixed url is left alone",
			target:     "/de/pricing",
			prefCookie: "de",
		},
		{
			name:       "default-language preference never redirects",
			target:     "/pricing",
			prefCookie: "en",
		},
		{
			name:   "no cookie means no redirect (crawlers)",
			target: "/pricing",
		},
		{
			name:       "stale/unknown cookie is ignored",
			target:     "/pricing",
			prefCookie: "xx",
		},
		{
			name:       "guest event pages resolve language from the event",
			target:     "/e/abc123",
			prefCookie: "de",
		},
		{
			name:       "non-html routes are not eligible",
			target:     "/sitemap.xml",
			prefCookie: "de",
		},
		{
			name:       "static assets are not eligible",
			target:     "/static/css/app.css",
			prefCookie: "de",
		},
		{
			name:       "api routes are not eligible",
			target:     "/api/user/tier",
			prefCookie: "de",
		},
		{
			name:       "non-GET requests are left alone",
			method:     http.MethodPost,
			target:     "/pricing?setlang=de",
			prefCookie: "de",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			req := httptest.NewRequest(method, tc.target, nil)
			gotSet, gotRedirect := langPreferenceAction(req, tc.prefCookie)
			if gotSet != tc.wantSet {
				t.Errorf("setCookie = %q, want %q", gotSet, tc.wantSet)
			}
			if gotRedirect != tc.wantRedirect {
				t.Errorf("redirect = %q, want %q", gotRedirect, tc.wantRedirect)
			}
		})
	}
}

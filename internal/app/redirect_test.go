package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// newGetEvent builds a minimal RequestEvent for a logged-out GET so the
// auth-gate redirect helpers can be exercised without booting PocketBase.
func newGetEvent(target string) (*core.RequestEvent, *httptest.ResponseRecorder) {
	e := &core.RequestEvent{}
	e.Request = httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	e.Response = rec
	return e, rec
}

// TestRedirectToRegisterPreservesQuery locks in the fix for logged-out users
// who click a checkout link: the URL they were headed to must survive the
// bounce through /register so they land back in checkout after signing up.
//
// The regression it guards against is raw string concatenation
// ("/register?redirect=" + RequestURI), which leaks the target's own "&"
// separators into the /register query and drops every parameter past the
// first — e.g. a campaign parameter, which would silently lose attribution.
func TestRedirectToRegisterPreservesQuery(t *testing.T) {
	cases := []struct {
		name         string
		target       string
		wantLocation string
		wantRedirect string
	}{
		{
			name:         "multi-param target keeps every param",
			target:       "/payment?plan=premium&campaign=spring",
			wantLocation: "/register?redirect=" + url.QueryEscape("/payment?plan=premium&campaign=spring"),
			wantRedirect: "/payment?plan=premium&campaign=spring",
		},
		{
			name:         "single-param target",
			target:       "/payment?plan=standard",
			wantLocation: "/register?redirect=" + url.QueryEscape("/payment?plan=standard"),
			wantRedirect: "/payment?plan=standard",
		},
		{
			name:         "localised target keeps its lang prefix and params",
			target:       "/de/payment?plan=premium&campaign=summer",
			wantLocation: "/de/register?redirect=" + url.QueryEscape("/de/payment?plan=premium&campaign=summer"),
			wantRedirect: "/de/payment?plan=premium&campaign=summer",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, rec := newGetEvent(tc.target)
			if err := redirectToRegister(e); err != nil {
				t.Fatalf("redirectToRegister: %v", err)
			}
			if rec.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
			}

			loc := rec.Header().Get("Location")
			if loc != tc.wantLocation {
				t.Errorf("Location = %q, want %q", loc, tc.wantLocation)
			}

			// Parse the redirect URL the way handleRegisterPage does and
			// confirm the original target is recovered whole.
			u, err := url.Parse(loc)
			if err != nil {
				t.Fatalf("parse Location: %v", err)
			}
			if got := u.Query().Get("redirect"); got != tc.wantRedirect {
				t.Errorf("recovered redirect = %q, want %q", got, tc.wantRedirect)
			}
			// The target's params must NOT have leaked into the /register
			// query as stray top-level parameters (the old bug).
			if stray := u.Query().Get("campaign"); stray != "" {
				t.Errorf("campaign leaked into /register query as %q", stray)
			}
			if stray := u.Query().Get("plan"); stray != "" {
				t.Errorf("plan leaked into /register query as %q", stray)
			}

			// The recovered redirect must be accepted by safeRedirect so the
			// post-signup hop actually forwards there.
			if got := safeRedirect(tc.wantRedirect, "/overview"); got != tc.wantRedirect {
				t.Errorf("safeRedirect rejected recovered target: got %q", got)
			}
		})
	}
}

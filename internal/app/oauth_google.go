package app

import (
	"log"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/auth"
	"github.com/pocketbase/pocketbase/tools/security"
	"golang.org/x/oauth2"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// Short-lived cookies that bridge the redirect to Google and back. They're
// set just before we bounce the user to the consent screen and cleared the
// moment we return. SameSite=Lax is required: the callback is a top-level
// GET navigation initiated by Google, and Lax (unlike Strict) still sends
// the cookies on that cross-site navigation.
const (
	oauthStateCookie    = "oauth_state"
	oauthVerifierCookie = "oauth_verifier"
	oauthRedirectCookie = "oauth_redirect"
	oauthLangCookie     = "oauth_lang"
)

// googleProvider builds a configured Google OAuth2 provider. The redirect URL
// is derived from app_url so it always matches the public origin printed on
// cards and registered in the Google Cloud console.
func googleProvider() (auth.Provider, error) {
	provider, err := auth.NewProviderByName(auth.NameGoogle)
	if err != nil {
		return nil, err
	}
	provider.SetClientId(appConfig.GoogleOAuth.ClientID)
	provider.SetClientSecret(appConfig.GoogleOAuth.ClientSecret)
	provider.SetRedirectURL(strings.TrimRight(appConfig.AppURL, "/") + "/auth/google/callback")
	return provider, nil
}

// handleGoogleLogin kicks off the Google OAuth2 flow: it mints a CSRF state +
// PKCE verifier, stashes them (and the post-login redirect target + language)
// in short-lived cookies, then sends the user to Google's consent screen.
func handleGoogleLogin(e *core.RequestEvent) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	prefix := i18n.LangPath(lang)

	if !appConfig.GoogleOAuth.Enabled() {
		return e.Redirect(http.StatusSeeOther, prefix+"/login?error=google_unavailable")
	}

	provider, err := googleProvider()
	if err != nil {
		return e.Redirect(http.StatusSeeOther, prefix+"/login?error=google_unavailable")
	}

	state := security.RandomString(30)
	verifier := security.RandomString(43)
	challenge := security.S256Challenge(verifier)

	setOAuthCookie(e, oauthStateCookie, state)
	setOAuthCookie(e, oauthVerifierCookie, verifier)
	setOAuthCookie(e, oauthLangCookie, lang)
	// Preserve where the user was headed (e.g. straight into checkout), the
	// same ?redirect= contract the email login/register handlers honour.
	setOAuthCookie(e, oauthRedirectCookie, safeRedirect(e.Request.URL.Query().Get("redirect"), ""))

	authURL := provider.BuildAuthURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return e.Redirect(http.StatusSeeOther, authURL)
}

// handleGoogleCallback completes the flow: it validates the state, exchanges
// the code for a token, fetches the verified Google profile and then logs the
// user in — creating the account on first sight (auth_provider="google") or
// linking to an existing one that already owns the same verified email.
func handleGoogleCallback(e *core.RequestEvent) error {
	// Recover language/redirect from the cookies first so every failure path
	// can bounce the user back to the right localised login page.
	lang := cookieValue(e, oauthLangCookie)
	if !i18n.IsSupported(lang) {
		lang = i18n.DefaultLang
	}
	prefix := i18n.LangPath(lang)
	redirect := safeRedirect(cookieValue(e, oauthRedirectCookie), prefix+"/overview")

	state := cookieValue(e, oauthStateCookie)
	verifier := cookieValue(e, oauthVerifierCookie)
	clearOAuthCookies(e)

	loginFail := prefix + "/login?error=google_failed"

	if !appConfig.GoogleOAuth.Enabled() {
		return e.Redirect(http.StatusSeeOther, prefix+"/login?error=google_unavailable")
	}

	q := e.Request.URL.Query()
	// User denied consent, or the temp cookies expired / didn't survive.
	if q.Get("error") != "" || state == "" || verifier == "" {
		return e.Redirect(http.StatusSeeOther, loginFail)
	}
	if q.Get("state") != state {
		return e.Redirect(http.StatusSeeOther, loginFail)
	}
	code := q.Get("code")
	if code == "" {
		return e.Redirect(http.StatusSeeOther, loginFail)
	}

	provider, err := googleProvider()
	if err != nil {
		return e.Redirect(http.StatusSeeOther, loginFail)
	}
	provider.SetContext(e.Request.Context())

	token, err := provider.FetchToken(code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		log.Printf("google oauth: token exchange failed: %v", err)
		return e.Redirect(http.StatusSeeOther, loginFail)
	}

	authUser, err := provider.FetchAuthUser(token)
	if err != nil {
		log.Printf("google oauth: fetch user failed: %v", err)
		return e.Redirect(http.StatusSeeOther, loginFail)
	}
	// Email is only populated when Google confirms the address is verified.
	// Without it we can't safely key an account, so refuse the sign-in.
	if authUser.Email == "" {
		return e.Redirect(http.StatusSeeOther, prefix+"/login?error=google_unverified")
	}

	record, err := findOrCreateGoogleUser(e, authUser, lang)
	if err != nil {
		log.Printf("google oauth: account provisioning failed: %v", err)
		return e.Redirect(http.StatusSeeOther, loginFail)
	}

	authToken, err := record.NewAuthToken()
	if err != nil {
		return e.InternalServerError("Failed to create auth token", err)
	}
	setAuthCookie(e, authToken)
	return e.Redirect(http.StatusSeeOther, redirect)
}

// findOrCreateGoogleUser returns the users record for the given Google profile,
// creating it on first login. An existing account that already owns the
// (verified) email is reused — linking, not duplicating — so a user who first
// signed up with email/password can later log in with Google seamlessly.
func findOrCreateGoogleUser(e *core.RequestEvent, authUser *auth.AuthUser, lang string) (*core.Record, error) {
	if record, err := e.App.FindAuthRecordByEmail("users", authUser.Email); err == nil && record != nil {
		return record, nil
	}

	collection, err := e.App.FindCollectionByNameOrId("users")
	if err != nil {
		return nil, err
	}

	record := core.NewRecord(collection)
	record.Set("email", authUser.Email)
	// OAuth accounts never use a password to sign in, but the auth collection
	// still expects one — set a strong random value that's never surfaced.
	record.Set("password", security.RandomString(30))
	record.Set("verified", true)
	record.Set("tier", "free")
	record.Set("auth_provider", "google")
	record.Set("signup_lang", lang)
	if authUser.Name != "" {
		record.Set("name", authUser.Name)
	}
	// Mirror the email signup path: a single best-effort, IP-based country
	// lookup for targeting. Failure leaves the field empty, never blocks signup.
	if country := lookupCountryByIP(e.Request.Context(), e.RealIP()); country != "" {
		record.Set("signup_country", country)
	}

	if err := e.App.Save(record); err != nil {
		return nil, err
	}
	return record, nil
}

// setOAuthCookie writes a 10-minute HttpOnly cookie used only between the
// consent redirect and the callback.
func setOAuthCookie(e *core.RequestEvent, name, value string) {
	e.SetCookie(&http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   strings.HasPrefix(appConfig.AppURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthCookies(e *core.RequestEvent) {
	for _, name := range []string{oauthStateCookie, oauthVerifierCookie, oauthRedirectCookie, oauthLangCookie} {
		e.SetCookie(&http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   0,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func cookieValue(e *core.RequestEvent, name string) string {
	if c, err := e.Request.Cookie(name); err == nil {
		return c.Value
	}
	return ""
}

package app

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/mails"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

func handleLoginPage(e *core.RequestEvent) error {
	data := map[string]any{
		"Error":    e.Request.URL.Query().Get("error"),
		"Notice":   e.Request.URL.Query().Get("notice"),
		"Redirect": e.Request.URL.Query().Get("redirect"),
	}
	return e.HTML(http.StatusOK, renderWithBase(e, "login", data))
}

func handleLoginSubmit(e *core.RequestEvent) error {
	data := struct {
		Identity string `json:"identity" form:"identity"`
		Password string `json:"password" form:"password"`
		Redirect string `json:"redirect" form:"redirect"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return e.BadRequestError("Invalid data", err)
	}

	lang, _ := i18n.FromPath(e.Request.URL.Path)
	prefix := i18n.LangPath(lang)

	redirectOnFail := prefix + "/login?error=invalid"
	if data.Redirect != "" {
		redirectOnFail += "&redirect=" + url.QueryEscape(data.Redirect)
	}

	record, err := e.App.FindAuthRecordByEmail("users", data.Identity)
	if err != nil {
		return e.Redirect(http.StatusSeeOther, redirectOnFail)
	}
	if !record.ValidatePassword(data.Password) {
		return e.Redirect(http.StatusSeeOther, redirectOnFail)
	}

	token, err := record.NewAuthToken()
	if err != nil {
		return e.InternalServerError("Failed to create auth token", err)
	}

	setAuthCookie(e, token)
	return e.Redirect(http.StatusSeeOther, safeRedirect(data.Redirect, prefix+"/overview"))
}

func handleRegisterPage(e *core.RequestEvent) error {
	data := map[string]any{
		"Error":    e.Request.URL.Query().Get("error"),
		"Redirect": e.Request.URL.Query().Get("redirect"),
	}
	return e.HTML(http.StatusOK, renderWithBase(e, "register", data))
}

func handleRegisterSubmit(e *core.RequestEvent) error {
	data := struct {
		Email           string `json:"email" form:"email"`
		Password        string `json:"password" form:"password"`
		PasswordConfirm string `json:"password_confirm" form:"password_confirm"`
		Redirect        string `json:"redirect" form:"redirect"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return e.BadRequestError("Invalid data", err)
	}

	lang, _ := i18n.FromPath(e.Request.URL.Path)
	prefix := i18n.LangPath(lang)

	if data.Password != data.PasswordConfirm {
		redirectOnFail := prefix + "/register?error=password_mismatch"
		if data.Redirect != "" {
			redirectOnFail += "&redirect=" + url.QueryEscape(data.Redirect)
		}
		return e.Redirect(http.StatusSeeOther, redirectOnFail)
	}

	collection, err := e.App.FindCollectionByNameOrId("users")
	if err != nil {
		return e.InternalServerError("Failed to find users collection", err)
	}

	record := core.NewRecord(collection)
	record.Set("email", data.Email)
	record.Set("password", data.Password)
	record.Set("tier", "free")
	record.Set("auth_provider", "email")
	record.Set("signup_lang", lang)
	// Resolve country synchronously: a single 3s-bounded HTTPS call to
	// ipapi.co. If it fails the field stays empty and the signup still
	// succeeds — country is for targeting, not auth.
	if country := lookupCountryByIP(e.Request.Context(), e.RealIP()); country != "" {
		record.Set("signup_country", country)
	}

	if err := e.App.Save(record); err != nil {
		redirectOnFail := prefix + classifyRegisterError(err.Error())
		if data.Redirect != "" {
			redirectOnFail += "&redirect=" + url.QueryEscape(data.Redirect)
		}
		return e.Redirect(http.StatusSeeOther, redirectOnFail)
	}

	token, err := record.NewAuthToken()
	if err != nil {
		return e.InternalServerError("Failed to create auth token", err)
	}

	setAuthCookie(e, token)
	return e.Redirect(http.StatusSeeOther, safeRedirect(data.Redirect, prefix+"/overview"))
}

// classifyRegisterError maps the underlying validation error string to a
// human-friendly query parameter. PocketBase wraps the original validator
// errors as a flat string, so substring matching is the pragmatic choice.
func classifyRegisterError(msg string) string {
	switch {
	case strings.Contains(msg, "email") && (strings.Contains(msg, "unique") || strings.Contains(msg, "already exists")):
		return "/register?error=email_taken"
	case strings.Contains(msg, "password") && (strings.Contains(msg, "min") || strings.Contains(msg, "length") || strings.Contains(msg, "short")):
		return "/register?error=short_password"
	case strings.Contains(msg, "email") && strings.Contains(msg, "invalid"):
		return "/register?error=invalid_email"
	default:
		return "/register?error=registration_failed"
	}
}

func handleLogout(e *core.RequestEvent) error {
	e.SetCookie(&http.Cookie{
		Name:     "pb_auth",
		Value:    "",
		Path:     "/",
		MaxAge:   0,
		HttpOnly: true,
		Secure:   strings.HasPrefix(appConfig.AppURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
	return redirectLocalised(e, http.StatusSeeOther, "/")
}

func handleForgotPasswordPage(e *core.RequestEvent) error {
	return e.HTML(http.StatusOK, renderWithBase(e, "forgot_password", map[string]any{
		"Status": e.Request.URL.Query().Get("status"),
	}))
}

func handleForgotPasswordSubmit(e *core.RequestEvent) error {
	data := struct {
		Email string `json:"email" form:"email"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return e.BadRequestError("Invalid data", err)
	}

	email := strings.TrimSpace(data.Email)
	if email != "" {
		if record, err := e.App.FindAuthRecordByEmail("users", email); err == nil && record != nil {
			if err := mails.SendRecordPasswordReset(e.App, record); err != nil {
				log.Printf("password reset email failed for %s: %v", email, err)
			}
		}
	}

	return redirectLocalised(e, http.StatusSeeOther, "/forgot-password?status=sent")
}

// setAuthCookie writes the pb_auth session cookie used by attachAuthFromCookie
// on subsequent requests. Lifetime matches PocketBase's default (~1 week).
// Secure is keyed off app_url (like the OAuth cookies) so the session token
// is never sent over plaintext HTTP in production while local dev still works.
func setAuthCookie(e *core.RequestEvent, token string) {
	e.SetCookie(&http.Cookie{
		Name:     "pb_auth",
		Value:    token,
		Path:     "/",
		MaxAge:   604800,
		HttpOnly: true,
		Secure:   strings.HasPrefix(appConfig.AppURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
}

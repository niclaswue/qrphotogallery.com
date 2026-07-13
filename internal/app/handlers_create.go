package app

import (
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// pendingEventCookie carries a filled-in create form across the register/login
// round-trip: an unauthenticated visitor can build their event first and only
// then is asked for an account, which converts far better than auth-first.
const pendingEventCookie = "pending_event"

// handleCreateStart renders the create form. Signed-in users with a pending
// cookie (bounced back from login) get it pre-filled.
func handleCreateStart(e *core.RequestEvent) error {
	title, eventDate, qrMode, designID, initialPrompts := readPendingCreate(e)
	initialPromptsJSON, _ := json.Marshal(initialPrompts)

	return e.HTML(http.StatusOK, renderWithBase(e, "create", map[string]any{
		"MaxPrompts":         getUserTier(e.Auth).MaxPrompts,
		"EventTitle":         title,
		"EventDate":          eventDate,
		"QRMode":             qrMode,
		"Designs":            Designs,
		"CurrentDesignID":    designID,
		"InitialPromptsJSON": template.JS(initialPromptsJSON),
		"FormAction":         "/create",
	}))
}

// normalizeQRMode collapses the create-form distribution choice to its two
// valid values. Anything unexpected falls back to the classic per-prompt
// cards so a tampered form can't put an event in an undefined state.
func normalizeQRMode(s string) string {
	if strings.TrimSpace(s) == "single" {
		return "single"
	}
	return "cards"
}

// handleCreateSubmit validates the create form. For signed-in users the event
// is created immediately; anonymous visitors get their input stashed in the
// pending cookie and are sent through registration, landing on /create/finish
// which completes the creation from the cookie.
func handleCreateSubmit(e *core.RequestEvent) error {
	data := struct {
		Title     string `json:"title" form:"title"`
		EventDate string `json:"event_date" form:"event_date"`
		Prompts   string `json:"prompts" form:"prompts"`
		QRMode    string `json:"qr_mode" form:"qr_mode"`
		DesignID  string `json:"design_id" form:"design_id"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.invalid_form")
	}

	title := strings.TrimSpace(data.Title)
	if title == "" {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.missing_gallery_name")
	}
	if len(title) > 120 {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.gallery_name_too_long")
	}
	eventDate := strings.TrimSpace(data.EventDate)
	if eventDate != "" && !isValidDate(eventDate) {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.invalid_date")
	}
	prompts := parsePrompts(data.Prompts)
	if len(prompts) == 0 {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.invalid_form", "error.message.upload_destination_not_found")
	}
	designID := data.DesignID
	if GetDesignByID(designID) == nil {
		designID = Designs[0].ID
	}

	if e.Auth == nil {
		// Not signed in: stash the form and round-trip through registration.
		writePendingCreate(e, title, eventDate, normalizeQRMode(data.QRMode), designID, prompts)
		redirect := url.QueryEscape("/create/finish")
		return redirectLocalised(e, http.StatusSeeOther, "/register?redirect="+redirect)
	}

	tier := getUserTier(e.Auth)
	if len(prompts) > tier.MaxPrompts {
		return renderUpgradeError(e, tier.MaxPrompts)
	}

	event, err := createEvent(e, title, eventDate, prompts, designID, normalizeQRMode(data.QRMode) == "single")
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusInternalServerError, "error.title.could_not_save", "error.message.could_not_save")
	}
	clearPendingCreate(e)
	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

// handleCreateFinish completes an event creation whose form data was stashed
// in the pending cookie before the register/login round-trip.
func handleCreateFinish(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	title, eventDate, qrMode, designID, prompts := readPendingCreate(e)
	if strings.TrimSpace(title) == "" || len(prompts) == 0 {
		return redirectLocalised(e, http.StatusSeeOther, "/create")
	}

	tier := getUserTier(e.Auth)
	if len(prompts) > tier.MaxPrompts {
		return renderUpgradeError(e, tier.MaxPrompts)
	}

	event, err := createEvent(e, title, eventDate, prompts, designID, qrMode == "single")
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusInternalServerError, "error.title.could_not_save", "error.message.could_not_save")
	}
	clearPendingCreate(e)
	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

// writePendingCreate stashes the validated create form in a short-lived
// cookie so it survives the register/login round-trip.
func writePendingCreate(e *core.RequestEvent, title, eventDate, qrMode, designID string, prompts []string) {
	pendingJSON, _ := json.Marshal(map[string]string{
		"title":      title,
		"event_date": eventDate,
		"qr_mode":    qrMode,
		"design_id":  designID,
		"prompts":    strings.Join(prompts, "\n"),
	})
	e.SetCookie(&http.Cookie{
		Name:     pendingEventCookie,
		Value:    base64.StdEncoding.EncodeToString(pendingJSON),
		Path:     "/",
		MaxAge:   3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// readPendingCreate decodes the pending cookie (if any) so the create page
// can pre-fill the form and /create/finish can complete the creation.
// Returns zero values if the cookie is missing or unreadable; the QR mode
// always normalises to "cards" or "single".
func readPendingCreate(e *core.RequestEvent) (title, eventDate, qrMode, designID string, prompts []string) {
	cookie, err := e.Request.Cookie(pendingEventCookie)
	if err != nil || cookie.Value == "" {
		return "", "", "cards", "", nil
	}
	decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return "", "", "cards", "", nil
	}
	var pending map[string]string
	if err := json.Unmarshal(decoded, &pending); err != nil {
		return "", "", "cards", "", nil
	}
	return pending["title"], pending["event_date"], normalizeQRMode(pending["qr_mode"]), pending["design_id"], parsePrompts(pending["prompts"])
}

func clearPendingCreate(e *core.RequestEvent) {
	e.SetCookie(&http.Cookie{Name: pendingEventCookie, Value: "", Path: "/", MaxAge: 0})
}

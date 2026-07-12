package app

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// handleEditPrompts renders the create form pre-populated with the event's
// current metadata and prompts so the owner can revise them. Reuses the
// create.html template via FormAction = /edit/{id}.
func handleEditPrompts(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	prompts, err := e.App.FindRecordsByFilter("prompts", "event = {:eid}", "sort_order", 1000, 0, dbxParams{"eid": event.Id})
	if err != nil {
		return e.InternalServerError("Failed to load prompts", err)
	}

	initialPrompts := make([]string, 0, len(prompts))
	initialPromptIDs := make([]string, 0, len(prompts))
	for _, p := range prompts {
		initialPrompts = append(initialPrompts, p.GetString("text"))
		initialPromptIDs = append(initialPromptIDs, p.Id)
	}
	initialPromptsJSON, _ := json.Marshal(initialPrompts)
	initialPromptIDsJSON, _ := json.Marshal(initialPromptIDs)

	return e.HTML(http.StatusOK, renderWithBase(e, "create", map[string]any{
		"MaxPrompts":           getUserTier(e.Auth).MaxPrompts,
		"EventTitle":           event.GetString("title"),
		"EventDate":            event.GetString("event_date"),
		"InitialPromptsJSON":   template.JS(initialPromptsJSON),
		"InitialPromptIDsJSON": template.JS(initialPromptIDsJSON),
		"FormAction":           "/edit/" + event.Id,
		"EditMode":             true,
		"EventID":              event.Id,
		"Designs":              Designs,
		"CurrentDesignID":      event.GetString("design_id"),
	}))
}

func handleEditPromptsSubmit(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	data := struct {
		Title       string `json:"title" form:"title"`
		EventDate   string `json:"event_date" form:"event_date"`
		Prompts     string `json:"prompts" form:"prompts"`
		PromptsJSON string `json:"prompts_json" form:"prompts_json"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return e.BadRequestError("Invalid data", err)
	}

	title := strings.TrimSpace(data.Title)
	if title == "" {
		return renderHTMLError(e, http.StatusBadRequest, "Missing Event Name", "Please enter a name for your event.")
	}
	if len(title) > 120 {
		return renderHTMLError(e, http.StatusBadRequest, "Event Name Too Long", "Event names can be at most 120 characters.")
	}

	eventDate := strings.TrimSpace(data.EventDate)
	if eventDate != "" && !isValidDate(eventDate) {
		return renderHTMLError(e, http.StatusBadRequest, "Invalid Date", "That date does not look right. Use the date picker or leave it empty.")
	}

	// The edit form posts prompts as JSON carrying each prompt's ID, so edits
	// are reconciled by identity (see updateEventPrompts). Fall back to the
	// plain newline field for any client that didn't send the JSON payload.
	prompts := parsePromptInputs(data.PromptsJSON)
	if prompts == nil {
		for _, text := range parsePrompts(data.Prompts) {
			prompts = append(prompts, promptInput{Text: text})
		}
	}
	// qrphotogallery.com hides the template's prompt model. The simple edit
	// form only updates gallery metadata, leaving the single internal upload
	// bucket (and every file attached to it) untouched.
	if len(prompts) > 0 {
		tier := getUserTier(e.Auth)
		if len(prompts) > tier.MaxPrompts {
			return renderUpgradeError(e, tier.MaxPrompts)
		}
	}

	event.Set("title", title)
	event.Set("event_date", eventDate)
	if err := e.App.Save(event); err != nil {
		return e.InternalServerError("Failed to update event", err)
	}

	if len(prompts) > 0 {
		if err := updateEventPrompts(e, event.Id, prompts); err != nil {
			return e.InternalServerError("Failed to update gallery", err)
		}
	}

	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

// handleEventSettingsSubmit toggles per-event guest-flow settings
// (lock_after_submit, disable_guest_download, collect_guest_name). Posted from
// the overview page; on success we land back on /overview/{id} so the owner
// sees the updated state immediately.
func handleEventSettingsSubmit(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}
	if !isPremiumTier(getUserTier(e.Auth).Name) {
		return renderHTMLError(e, http.StatusForbidden, "Commercial Feature", "Advanced guest controls are included in the Commercial plan.")
	}

	data := struct {
		LockAfterSubmit      string `json:"lock_after_submit" form:"lock_after_submit"`
		DisableGuestDownload string `json:"disable_guest_download" form:"disable_guest_download"`
		CollectGuestName     string `json:"collect_guest_name" form:"collect_guest_name"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return e.BadRequestError("Invalid data", err)
	}

	event.Set("lock_after_submit", isCheckboxOn(data.LockAfterSubmit))
	event.Set("disable_guest_download", isCheckboxOn(data.DisableGuestDownload))
	event.Set("collect_guest_name", isCheckboxOn(data.CollectGuestName))
	if err := e.App.Save(event); err != nil {
		return e.InternalServerError("Failed to save settings", err)
	}

	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

// handleToggleQRMode switches an event between the two distribution modes:
// classic per-prompt cards ("cards") and the single shared QR code ("single").
// It lives on its own endpoint (rather than the paid guest-settings form)
// because choosing a mode is free for every tier, so all owners can switch.
func handleToggleQRMode(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	data := struct {
		QRMode string `json:"qr_mode" form:"qr_mode"`
	}{}
	if err := e.BindBody(&data); err != nil {
		return e.BadRequestError("Invalid data", err)
	}

	event.Set("single_qr_mode", normalizeQRMode(data.QRMode) == "single")
	if err := e.App.Save(event); err != nil {
		return e.InternalServerError("Failed to save settings", err)
	}

	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

// handleEventLangSubmit sets the event's language — what guests land on by
// default and the language of the printed cards/poster. Language is free for
// every tier, so this lives on its own endpoint like the QR-mode toggle.
func handleEventLangSubmit(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	if err := e.Request.ParseForm(); err != nil {
		return e.BadRequestError("Invalid data", err)
	}
	lang := e.Request.FormValue("lang")
	if !i18n.IsSupported(lang) {
		return e.BadRequestError("Unsupported language", nil)
	}

	event.Set("lang", lang)
	if err := e.App.Save(event); err != nil {
		return e.InternalServerError("Failed to save language", err)
	}

	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id)
}

// handleDeleteEvent removes an event together with its prompts and uploads.
// Owner-only. The uploads' stored files are removed by PocketBase when their
// records are deleted.
func handleDeleteEvent(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	err = e.App.RunInTransaction(func(txApp core.App) error {
		prompts, err := txApp.FindRecordsByFilter("prompts", "event = {:eid}", "", 0, 0, dbxParams{"eid": event.Id})
		if err != nil {
			return err
		}
		for _, p := range prompts {
			if err := deletePromptUploads(txApp, p.Id); err != nil {
				return err
			}
			if err := txApp.Delete(p); err != nil {
				return err
			}
		}
		return txApp.Delete(event)
	})
	if err != nil {
		return e.InternalServerError("Failed to delete event", err)
	}

	return redirectLocalised(e, http.StatusSeeOther, "/overview")
}

// isCheckboxOn returns true for the canonical "checked" form values. HTML
// forms only send the field when the box is ticked, so absent strings are
// treated as false.
func isCheckboxOn(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "1", "true", "yes":
		return true
	}
	return false
}

func handleChangeDesignSubmit(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectLocalised(e, http.StatusSeeOther, "/register")
	}

	id := e.Request.PathValue("id")
	designID := e.Request.URL.Query().Get("design_id")
	if designID == "" {
		data := struct {
			DesignID string `json:"design_id" form:"design_id"`
		}{}
		if err := e.BindBody(&data); err == nil && data.DesignID != "" {
			designID = data.DesignID
		}
	}

	if GetDesignByID(designID) == nil {
		return e.BadRequestError("Invalid design selected", nil)
	}

	if err := updateEventDesign(e, id, designID); err != nil {
		return e.InternalServerError("Failed to update design", err)
	}

	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+id)
}

func handleGetUserTier(e *core.RequestEvent) error {
	if e.Auth == nil {
		return e.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	tier := getUserTier(e.Auth)
	return e.JSON(http.StatusOK, map[string]any{
		"tier":        tier.Name,
		"max_prompts": tier.MaxPrompts,
	})
}

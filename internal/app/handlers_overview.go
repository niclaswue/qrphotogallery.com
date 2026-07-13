package app

import (
	"fmt"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// handleOverviewList renders the signed-in user's events, newest first, with
// a create CTA. Users with no events yet are sent straight to /create.
func handleOverviewList(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectLocalised(e, http.StatusSeeOther, "/register")
	}

	events := findOwnedEvents(e)
	if len(events) == 0 {
		return redirectLocalised(e, http.StatusSeeOther, "/create")
	}

	type eventRow struct {
		ID          string
		Title       string
		EventDate   string
		SingleQR    bool
		PromptCount int
		UploadCount int
	}
	rows := make([]eventRow, 0, len(events))
	for _, ev := range events {
		prompts, _ := e.App.FindRecordsByFilter("prompts", "event = {:eid}", "", 0, 0, dbxParams{"eid": ev.Id})
		uploads, _ := e.App.FindRecordsByFilter("uploads", "event = {:eid}", "", 0, 0, dbxParams{"eid": ev.Id})
		rows = append(rows, eventRow{
			ID:          ev.Id,
			Title:       ev.GetString("title"),
			EventDate:   ev.GetString("event_date"),
			SingleQR:    ev.GetBool("single_qr_mode"),
			PromptCount: len(prompts),
			UploadCount: len(uploads),
		})
	}

	return e.HTML(http.StatusOK, renderWithBase(e, "overview_list", map[string]any{
		"Events": rows,
	}))
}

// handleOverview renders the event dashboard for the owner: prompt list,
// upload status per prompt, QR/print downloads and the guest-flow settings.
func handleOverview(e *core.RequestEvent) error {
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

	type promptWithUpload struct {
		ID       string
		Text     string
		HasImage bool
	}

	var pList []promptWithUpload
	for _, p := range prompts {
		uploads, _ := e.App.FindRecordsByFilter("uploads", "prompt = {:pid}", "-created", 1, 0, dbxParams{"pid": p.Id})
		pList = append(pList, promptWithUpload{
			ID:       p.Id,
			Text:     p.GetString("text"),
			HasImage: len(uploads) > 0,
		})
	}

	design := GetDesignByID(event.GetString("design_id"))
	if design == nil {
		design = &Designs[0]
	}
	uploads, _ := e.App.FindRecordsByFilter("uploads", "event = {:eid}", "", 0, 0, dbxParams{"eid": event.Id})
	usedGB := float64(galleryUsageBytes(e.App, event.Id)) / float64(1024*1024*1024)
	expires := event.GetDateTime("created").Time().AddDate(1, 0, 0)
	lang, _ := i18n.FromPath(e.Request.URL.Path)

	return e.HTML(http.StatusOK, renderWithBase(e, "overview", map[string]any{
		"Event":                event,
		"Design":               design,
		"Designs":              Designs,
		"Prompts":              pList,
		"LockAfterSubmit":      event.GetBool("lock_after_submit"),
		"DisableGuestDownload": event.GetBool("disable_guest_download"),
		"CollectGuestName":     event.GetBool("collect_guest_name"),
		"SingleQRMode":         event.GetBool("single_qr_mode"),
		"EventLang":            eventStoredLang(event),
		"SupportEmail":         appConfig.SupportEmail,
		"UploadCount":          len(uploads),
		"StorageUsedGB":        fmt.Sprintf("%.2f", usedGB),
		"DisplayEventDate":     formatDisplayDate(event.GetString("event_date"), lang),
		"ExpiresOn":            formatDisplayTime(expires, lang),
	}))
}

// handleGallery renders every upload for an event, grouped by prompt.
// Empty prompts still appear so the owner can see what's missing.
func handleGallery(e *core.RequestEvent) error {
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

	type galleryItem struct {
		PromptID   string
		PromptText string
		ImageURL   string
		UploadID   string
		GuestName  string
		MediaKind  string
	}

	var items []galleryItem
	hasUploads := false
	for _, p := range prompts {
		promptText := p.GetString("text")
		uploads, _ := e.App.FindRecordsByFilter("uploads", "prompt = {:pid}", "-created", 0, 0, dbxParams{"pid": p.Id})
		if len(uploads) == 0 {
			items = append(items, galleryItem{PromptID: p.Id, PromptText: promptText})
			continue
		}
		hasUploads = true
		for _, u := range uploads {
			items = append(items, galleryItem{
				PromptID:   p.Id,
				PromptText: promptText,
				ImageURL:   uploadDisplayURL(u),
				UploadID:   u.Id,
				GuestName:  u.GetString("guest_name"),
				MediaKind:  uploadMediaKind(u),
			})
		}
	}

	return e.HTML(http.StatusOK, renderWithBase(e, "gallery", map[string]any{
		"Event":      event,
		"Items":      items,
		"HasUploads": hasUploads,
	}))
}

// handleDeleteUpload lets an event owner remove a single upload from the
// gallery — for moderating a submission that shouldn't be there. Deleting
// the upload record also removes its stored files. Owner-only; redirects back
// to the gallery so the change is visible immediately.
func handleDeleteUpload(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	id := e.Request.PathValue("id")
	upload, err := e.App.FindRecordById("uploads", id)
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.upload_not_found")
	}

	eventID := upload.GetString("event")
	event, err := e.App.FindRecordById("events", eventID)
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	if e.Auth.Id != event.GetString("owner") {
		return renderHTMLErrorKeys(e, http.StatusForbidden, "error.title.not_authorized", "error.message.upload_access")
	}

	if err := e.App.Delete(upload); err != nil {
		return e.InternalServerError("Failed to delete upload", err)
	}

	return redirectLocalised(e, http.StatusSeeOther, "/gallery/"+eventID)
}

package app

import (
	"fmt"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

type galleryMediaItem struct {
	ImageURL  string
	UploadID  string
	GuestName string
	MediaKind string
}

func galleryMediaItems(app core.App, eventID string) []galleryMediaItem {
	uploads, err := app.FindRecordsByFilter("uploads", "event = {:eid}", "-created", 10000, 0, dbxParams{"eid": eventID})
	if err != nil {
		return nil
	}
	items := make([]galleryMediaItem, 0, len(uploads))
	for _, upload := range uploads {
		items = append(items, galleryMediaItem{
			ImageURL:  uploadDisplayURL(upload),
			UploadID:  upload.Id,
			GuestName: upload.GetString("guest_name"),
			MediaKind: uploadMediaKind(upload),
		})
	}
	return items
}

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
		UploadCount int
	}
	rows := make([]eventRow, 0, len(events))
	for _, event := range events {
		uploads, _ := e.App.FindRecordsByFilter("uploads", "event = {:eid}", "", 0, 0, dbxParams{"eid": event.Id})
		rows = append(rows, eventRow{
			ID:          event.Id,
			Title:       event.GetString("title"),
			EventDate:   event.GetString("event_date"),
			UploadCount: len(uploads),
		})
	}
	return e.HTML(http.StatusOK, renderWithBase(e, "overview_list", map[string]any{"Events": rows}))
}

// handleOverview is the whole host workspace: the one QR code, gallery stats,
// settings, and a flat view of every uploaded file. The hidden bucket never
// leaks into this page.
func handleOverview(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	items := galleryMediaItems(e.App, event.Id)
	usedGB := float64(galleryUsageBytes(e.App, event.Id)) / float64(1024*1024*1024)
	expires := event.GetDateTime("created").Time().AddDate(1, 0, 0)
	lang, _ := i18n.FromPath(e.Request.URL.Path)

	return e.HTML(http.StatusOK, renderWithBase(e, "overview", map[string]any{
		"Event":                event,
		"Items":                items,
		"HasUploads":           len(items) > 0,
		"DisableGuestDownload": event.GetBool("disable_guest_download"),
		"CollectGuestName":     event.GetBool("collect_guest_name"),
		"EventLang":            eventStoredLang(event),
		"SupportEmail":         appConfig.SupportEmail,
		"UploadCount":          len(items),
		"StorageUsedGB":        fmt.Sprintf("%.2f", usedGB),
		"DisplayEventDate":     formatDisplayDate(event.GetString("event_date"), lang),
		"ExpiresOn":            formatDisplayTime(expires, lang),
	}))
}

// Old owner-gallery bookmarks now land on the uploads section of the unified
// dashboard.
func handleLegacyOwnerGallery(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}
	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+event.Id+"#uploads")
}

func handleDeleteUpload(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	upload, err := e.App.FindRecordById("uploads", e.Request.PathValue("id"))
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
		return renderHTMLErrorKeys(e, http.StatusInternalServerError, "error.title.could_not_save", "error.message.could_not_save")
	}
	return redirectLocalised(e, http.StatusSeeOther, "/overview/"+eventID+"#uploads")
}

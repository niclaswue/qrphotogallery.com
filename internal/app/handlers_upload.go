package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

func guestLang(e *core.RequestEvent, event *core.Record) string {
	if urlLang, _ := i18n.FromPath(e.Request.URL.Path); urlLang != i18n.DefaultLang {
		return urlLang
	}
	if pinned := e.Request.URL.Query().Get("lang"); i18n.IsSupported(pinned) {
		return pinned
	}
	return eventStoredLang(event)
}

// handleEventDispatch is the complete guest experience at the one URL encoded
// by the event QR. GET renders the uploader and every existing file; POST adds
// a batch to the single hidden bucket and returns to that same gallery.
func handleEventDispatch(e *core.RequestEvent) error {
	eventID := e.Request.PathValue("id")
	if eventID == "" {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	event, err := e.App.FindRecordById("events", eventID)
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	lang := guestLang(e, event)
	if !eventGalleryActive(event) {
		return renderHTMLErrorKeysLang(e, http.StatusGone, lang, "error.title.gallery_expired", "error.message.gallery_expired")
	}
	bucket, err := findGalleryBucket(e.App, eventID)
	if err != nil {
		return renderHTMLErrorKeysLang(e, http.StatusNotFound, lang, "error.title.not_found", "error.message.upload_destination_not_found")
	}

	collectName := collectGuestNameEnabled(e.App, event)
	ownerPaid := eventOwnerPaid(e.App, event)
	guestName := strings.TrimSpace(e.Request.FormValue("guest_name"))

	renderGallery := func(errorKey string) error {
		items := galleryMediaItems(e.App, eventID)
		return e.HTML(http.StatusOK, renderWithBase(e, "upload", map[string]any{
			"GuestPage":            true,
			"Lang":                 lang,
			"Event":                event,
			"Items":                items,
			"Slides":               items,
			"UploadedCount":        len(items),
			"GuestDownloadEnabled": ownerPaid && !disableGuestDownloadEnabled(e.App, event),
			"CanBatch":             ownerPaid,
			"CollectName":          collectName,
			"GuestName":            guestName,
			"ErrorKey":             errorKey,
			"UploadSuccess":        e.Request.URL.Query().Get("uploaded") == "1",
		}))
	}

	if e.Request.Method == http.MethodGet {
		return renderGallery("")
	}

	files, err := e.FindUploadedFiles("image")
	if err != nil || len(files) == 0 {
		return renderGallery("upload.error.no_file")
	}
	if len(files) > 100 {
		return renderGallery("upload.error.too_many")
	}

	// The free account is a functional one-file preview. Both paid offers
	// unlock batch and repeat uploads up to the advertised 100 GB allowance.
	if !ownerPaid {
		existing, _ := e.App.FindRecordsByFilter("uploads", "event = {:eid}", "", 2, 0, dbxParams{"eid": eventID})
		if len(existing)+len(files) > 1 {
			return renderGallery("upload.error.free_limit")
		}
	}

	formats := make([]string, len(files))
	var incomingBytes int64
	for i, file := range files {
		if file.Size <= 0 || file.Size > maxUploadFileSize {
			return renderGallery("upload.error.too_large")
		}
		format, _, ok := detectUploadFormat(file)
		if !ok {
			return renderGallery("upload.error.bad_format")
		}
		formats[i] = format
		incomingBytes += file.Size
	}
	if galleryUsageBytes(e.App, eventID)+incomingBytes > galleryStorageLimit {
		return renderGallery("upload.error.gallery_full")
	}

	if collectName {
		if guestName == "" {
			return renderGallery("upload.error.name_required")
		}
		if len([]rune(guestName)) > maxGuestNameLen {
			guestName = string([]rune(guestName)[:maxGuestNameLen])
		}
	}

	uploads, err := e.App.FindCollectionByNameOrId("uploads")
	if err != nil {
		return e.InternalServerError("Failed to create upload record", err)
	}
	for i, file := range files {
		record := core.NewRecord(uploads)
		record.Set("prompt", bucket.Id)
		record.Set("event", eventID)
		record.Set("image", file)
		if collectName {
			record.Set("guest_name", guestName)
		}
		if err := e.App.Save(record); err != nil {
			return e.InternalServerError("Failed to save upload", err)
		}
		if formats[i] == "heic" || formats[i] == "heif" {
			queueDisplayConversion(e.App, record.Id)
		}
	}

	return redirectLocalised(e, http.StatusSeeOther, fmt.Sprintf("/e/%s?uploaded=1#gallery", eventID))
}

// Compatibility redirects keep previously shared gallery/library links useful
// while making the canonical guest surface the single QR URL.
func handleLegacyGuestGallery(e *core.RequestEvent) error {
	event, err := e.App.FindRecordById("events", e.Request.PathValue("id"))
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	return e.Redirect(http.StatusSeeOther, i18n.LangPath(guestLang(e, event))+"/e/"+event.Id+"#gallery")
}

func handleLegacyPromptLink(e *core.RequestEvent) error {
	event, err := e.App.FindRecordById("events", e.Request.PathValue("id"))
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	return e.Redirect(http.StatusSeeOther, i18n.LangPath(guestLang(e, event))+"/e/"+event.Id)
}

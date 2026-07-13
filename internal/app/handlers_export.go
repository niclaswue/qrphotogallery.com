package app

import (
	"fmt"
	"net/http"

	"github.com/pocketbase/pocketbase/core"
	"github.com/skip2/go-qrcode"
)

func handlePrintPoster(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}
	return generatePosterPDF(e, event, eventStoredLang(event))
}

func findOwnedEvent(e *core.RequestEvent) (*core.Record, error) {
	id := e.Request.PathValue("id")
	event, err := e.App.FindRecordById("events", id)
	if err != nil {
		return nil, renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	if e.Auth == nil || e.Auth.Id != event.GetString("owner") {
		return nil, renderHTMLErrorKeys(e, http.StatusForbidden, "error.title.not_authorized", "error.message.event_access")
	}
	return event, nil
}

// Every gallery has exactly one QR. The PNG remains available on the free
// preview so the core scan experience can be tested before purchase.
func handleDownloadQRImage(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}
	lang := eventStoredLang(event)
	target := fmt.Sprintf("%s/e/%s?lang=%s", appConfig.AppURL, event.Id, lang)
	png, err := qrcode.Encode(target, qrcode.Medium, 1024)
	if err != nil {
		return e.InternalServerError("Failed to render QR code", err)
	}
	e.Response.Header().Set("Content-Type", "image/png")
	e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-qr.png\"", sanitizeFilename(event.GetString("title"))))
	_, err = e.Response.Write(png)
	return err
}

func handleGuestDownloadGallery(e *core.RequestEvent) error {
	event, err := e.App.FindRecordById("events", e.Request.PathValue("id"))
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	lang := guestLang(e, event)
	if !eventGalleryActive(event) {
		return renderHTMLErrorKeysLang(e, http.StatusGone, lang, "error.title.gallery_expired", "error.message.gallery_expired")
	}
	if !eventOwnerPaid(e.App, event) {
		return renderHTMLErrorKeysLang(e, http.StatusForbidden, lang, "error.title.not_available", "error.message.guest_download_paid")
	}
	if disableGuestDownloadEnabled(e.App, event) {
		return renderHTMLErrorKeysLang(e, http.StatusForbidden, lang, "error.title.not_available", "error.message.guest_download_disabled")
	}
	uploads, err := e.App.FindRecordsByFilter("uploads", "event = {:eid}", "-created", 1, 0, dbxParams{"eid": event.Id})
	if err != nil {
		return e.InternalServerError("Failed to load uploads", err)
	}
	if len(uploads) == 0 {
		return renderHTMLErrorKeysLang(e, http.StatusBadRequest, lang, "error.title.no_uploads", "error.message.no_uploads_guest")
	}
	return downloadGalleryZip(e, event)
}

func handleDownloadGallery(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}
	uploads, err := e.App.FindRecordsByFilter("uploads", "event = {:eid}", "-created", 1, 0, dbxParams{"eid": event.Id})
	if err != nil {
		return e.InternalServerError("Failed to load uploads", err)
	}
	if len(uploads) == 0 {
		return renderHTMLErrorKeys(e, http.StatusBadRequest, "error.title.no_uploads", "error.message.no_uploads_owner")
	}
	return downloadGalleryZip(e, event)
}

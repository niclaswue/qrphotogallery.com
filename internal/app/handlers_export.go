package app

import (
	"archive/zip"
	"fmt"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/skip2/go-qrcode"
)

// handlePrintCards renders a Typst-generated PDF of upload cards (one per
// prompt, each with a QR code) for the owner to print and place at venues.
func handlePrintCards(e *core.RequestEvent) error {
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

	if len(prompts) == 0 {
		return renderHTMLError(e, http.StatusBadRequest, "No Prompts", "This event has no prompts. Add prompts before printing cards.")
	}

	design := GetDesignByID(event.GetString("design_id"))
	if design == nil {
		design = &Designs[0]
	}

	return generateCardsPDF(e, event, prompts, design, eventStoredLang(event))
}

// handlePrintPoster renders the single-QR poster PDF (one page, one big QR
// pointing at app_url/e/{id}) for the single-QR distribution mode. Owner-only,
// like handlePrintCards.
func handlePrintPoster(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}

	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	design := GetDesignByID(event.GetString("design_id"))
	if design == nil {
		design = &Designs[0]
	}

	return generatePosterPDF(e, event, design, eventStoredLang(event))
}

// findOwnedEvent loads the event for the request's {id} path value and
// verifies the authenticated user owns it. Returns a nil record after writing
// the error response when the lookup or ownership check fails.
func findOwnedEvent(e *core.RequestEvent) (*core.Record, error) {
	id := e.Request.PathValue("id")
	event, err := e.App.FindRecordById("events", id)
	if err != nil {
		return nil, renderHTMLError(e, http.StatusNotFound, "Not Found", "Event not found.")
	}
	if e.Auth.Id != event.GetString("owner") {
		return nil, renderHTMLError(e, http.StatusForbidden, "Not Authorized", "You do not have access to this event.")
	}
	return event, nil
}

// handleDownloadQRImage serves the bare single QR code as a PNG (pointing at
// the dispatcher, app_url/e/{id}) so paying owners can design their own
// poster or signs instead of using the built-in PDF.
func handleDownloadQRImage(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	if !isPaidTier(getUserTier(e.Auth).Name) {
		return renderQRDownloadLockedError(e)
	}

	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	// Pin the event language into the QR target with ?lang= so a scan lands
	// on the upload flow in that language, matching the printed cards.
	lang := eventStoredLang(event)
	png, err := qrcode.Encode(fmt.Sprintf("%s/e/%s?lang=%s", appConfig.AppURL, event.Id, lang), qrcode.Medium, 1024)
	if err != nil {
		return e.InternalServerError("Failed to render QR code", err)
	}

	e.Response.Header().Set("Content-Type", "image/png")
	e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-qr-%s.png\"", sanitizeFilename(event.GetString("title")), lang))
	_, err = e.Response.Write(png)
	return err
}

// handleDownloadQRZip streams a ZIP with one bare QR PNG per prompt (each
// pointing at that prompt's upload page), named by sort order + prompt text so
// the archive sorts naturally. The DIY counterpart of the printed cards PDF.
func handleDownloadQRZip(e *core.RequestEvent) error {
	if e.Auth == nil {
		return redirectToRegister(e)
	}
	if !isPaidTier(getUserTier(e.Auth).Name) {
		return renderQRDownloadLockedError(e)
	}

	event, err := findOwnedEvent(e)
	if event == nil {
		return err
	}

	prompts, err := e.App.FindRecordsByFilter("prompts", "event = {:eid}", "sort_order", 1000, 0, dbxParams{"eid": event.Id})
	if err != nil {
		return e.InternalServerError("Failed to load prompts", err)
	}
	if len(prompts) == 0 {
		return renderHTMLError(e, http.StatusBadRequest, "No Prompts", "This event has no prompts. Add prompts before downloading QR codes.")
	}

	lang := eventStoredLang(event)
	e.Response.Header().Set("Content-Type", "application/zip")
	e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-qr-codes-%s.zip\"", sanitizeFilename(event.GetString("title")), lang))

	zipWriter := zip.NewWriter(e.Response)
	defer zipWriter.Close()

	// Same ?lang= pin as the printed cards so every downloaded QR points at the
	// upload page in the event's language.
	for _, p := range prompts {
		png, err := qrcode.Encode(fmt.Sprintf("%s/e/%s/%s?lang=%s", appConfig.AppURL, event.Id, p.Id, lang), qrcode.Medium, 1024)
		if err != nil {
			return e.InternalServerError("Failed to render QR code", err)
		}
		w, err := zipWriter.Create(fmt.Sprintf("%02d-%s.png", p.GetInt("sort_order"), zipEntryName(p.GetString("text"))))
		if err != nil {
			return err
		}
		if _, err := w.Write(png); err != nil {
			return err
		}
	}
	return nil
}

// zipEntryName makes a prompt text safe as a ZIP member name: path separators
// and control characters are replaced and the result is capped so archive
// listings stay readable. Unlike Content-Disposition filenames, ZIP entries
// may keep non-ASCII characters.
func zipEntryName(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		out = "prompt"
	}
	if runes := []rune(out); len(runes) > 50 {
		out = string(runes[:50])
	}
	return out
}

// handleGuestDownloadGallery is the unauthenticated counterpart of
// handleDownloadGallery: anyone with the QR-shared event ID can grab
// the ZIP unless the owner has flipped disable_guest_download to true.
// Owners always have the authenticated /download/{id} route.
func handleGuestDownloadGallery(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	event, err := e.App.FindRecordById("events", id)
	if err != nil {
		return renderHTMLError(e, http.StatusNotFound, "Not Found", "Event not found.")
	}
	if !eventGalleryActive(event) {
		return renderHTMLError(e, http.StatusGone, "Gallery Expired", "This gallery's one-year availability period has ended.")
	}
	// Guest downloads are a paid feature; the owner-only /download/{id} route
	// stays open so hosts can always retrieve their own photos.
	if !eventOwnerPaid(e.App, event) {
		return renderHTMLError(e, http.StatusForbidden, "Not Available", "Guest gallery downloads are available on the host's paid plan only.")
	}
	if disableGuestDownloadEnabled(e.App, event) {
		return renderHTMLError(e, http.StatusForbidden, "Not Available", "The host has disabled guest downloads for this gallery.")
	}

	uploads, err := e.App.FindRecordsByFilter("uploads", "event = {:eid}", "-created", 1, 0, dbxParams{"eid": event.Id})
	if err != nil {
		return e.InternalServerError("Failed to load uploads", err)
	}
	if len(uploads) == 0 {
		return renderHTMLError(e, http.StatusBadRequest, "No Uploads", "No photos uploaded yet.")
	}

	return downloadGalleryZip(e, event)
}

// handleDownloadGallery streams a ZIP of every upload for an event.
// Filenames are derived from the prompt sort order so the archive is
// naturally sorted on disk.
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
		return renderHTMLError(e, http.StatusBadRequest, "No Uploads", "No uploads yet. Share your event link with guests first.")
	}

	return downloadGalleryZip(e, event)
}

package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// guestLang resolves the language for a guest-facing page: an explicit URL
// prefix (/de/e/...) or ?lang= pin (printed QR codes carry one) wins,
// otherwise the event's stored language. Guests never log in, so this is the
// whole negotiation.
func guestLang(e *core.RequestEvent, event *core.Record) string {
	if urlLang, _ := i18n.FromPath(e.Request.URL.Path); urlLang != i18n.DefaultLang {
		return urlLang
	}
	if q := e.Request.URL.Query().Get("lang"); i18n.IsSupported(q) {
		return q
	}
	return eventStoredLang(event)
}

// submissionCookieName returns the per-event cookie name we set after a
// successful guest upload. Used by the soft-lock check below and by the
// library page to render the "thanks, you've already submitted" toast.
func submissionCookieName(eventID string) string {
	return "guest_submitted_" + eventID
}

// guestHasSubmitted reports whether the request carries the submission
// cookie for the given event.
func guestHasSubmitted(e *core.RequestEvent, eventID string) bool {
	c, err := e.Request.Cookie(submissionCookieName(eventID))
	return err == nil && c != nil && c.Value != ""
}

// setSubmissionCookie marks the guest's browser as having uploaded once. The
// 180-day lifetime covers the event day plus a long after-party tail; the
// cookie is scoped to / so it works for every prompt URL of this event.
func setSubmissionCookie(e *core.RequestEvent, eventID string) {
	http.SetCookie(e.Response, &http.Cookie{
		Name:     submissionCookieName(eventID),
		Value:    "1",
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 180,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

// handleEventUpload renders the per-prompt upload page on GET and accepts a
// single image upload on POST. The route is unauthenticated by design —
// guests scan a QR code on a printed card and submit photos without
// creating an account.
func handleEventUpload(e *core.RequestEvent) error {
	eventID := e.Request.PathValue("id")
	promptIDFromURL := e.Request.PathValue("promptID")

	if eventID == "" {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}

	event, err := e.App.FindRecordById("events", eventID)
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}

	lang := guestLang(e, event)

	prompts, err := e.App.FindRecordsByFilter("prompts", "event = {:eid}", "sort_order", 1000, 0, dbxParams{"eid": eventID})
	if err != nil || len(prompts) == 0 {
		return renderHTMLErrorKeysLang(e, http.StatusNotFound, lang, "error.title.not_found", "error.message.upload_destination_not_found")
	}

	var prompt *core.Record
	promptNumber := 0
	for i, p := range prompts {
		if p.Id == promptIDFromURL {
			prompt = p
			promptNumber = i + 1
			break
		}
	}
	if prompt == nil {
		return renderHTMLErrorKeysLang(e, http.StatusNotFound, lang, "error.title.not_found", "error.message.upload_destination_not_found")
	}

	// Soft lock: if the owner has enabled the paid one-upload-per-guest lock
	// and this guest's browser already submitted to this event, redirect
	// to the library instead of rendering an upload form. Owners previewing
	// their own event bypass the lock — they're authenticated.
	if lockAfterSubmitEnabled(e.App, event) && guestHasSubmitted(e, eventID) && (e.Auth == nil || e.Auth.Id != event.GetString("owner")) {
		return redirectLocalised(e, http.StatusSeeOther, fmt.Sprintf("/e/%s/library?already=1", eventID))
	}

	design := GetDesignByID(event.GetString("design_id"))
	if design == nil {
		design = &Designs[0]
	}

	// In single-QR mode the guest reached this prompt via the dispatcher and
	// the form carries mode=qr. We then re-render the reveal view on
	// validation errors (and redirect to the QR-aware done page on success)
	// instead of the classic per-prompt upload page.
	qrMode := e.Request.Method == http.MethodPost && e.Request.FormValue("mode") == "qr"

	// When the owner has enabled "ask for a name", the form shows a required
	// name field. We read the submitted value up front so it can be echoed back
	// on a re-render (validation error) and saved on success.
	collectName := collectGuestNameEnabled(e.App, event)
	guestName := strings.TrimSpace(e.Request.FormValue("guest_name"))

	renderUpload := func(errKey string) error {
		page := "upload"
		if qrMode {
			page = "challenge"
		}
		return e.HTML(http.StatusOK, renderWithBase(e, page, map[string]any{
			"GuestPage":    true,
			"Lang":         lang,
			"Event":        event,
			"Design":       design,
			"Prompt":       prompt,
			"PromptID":     prompt.Id,
			"PromptText":   prompt.GetString("text"),
			"PromptNumber": promptNumber,
			"TotalPrompts": len(prompts),
			"ErrorKey":     errKey,
			"SingleQR":     qrMode,
			"CollectName":  collectName,
			"GuestName":    guestName,
		}))
	}

	if !eventGalleryActive(event) {
		return renderUpload("upload.error.expired")
	}

	if e.Request.Method == http.MethodGet {
		return renderUpload("")
	}

	// Multiple uploads per prompt is a paid feature. On the free plan each
	// prompt keeps a single upload, so once one exists we don't stack more —
	// the guest gets a short note pointing at the gallery instead. Owners on
	// a paid plan skip this and guests can keep adding photos.
	if !eventOwnerPaid(e.App, event) {
		existing, _ := e.App.FindRecordsByFilter("uploads", "prompt = {:pid}", "-created", 1, 0, dbxParams{"pid": prompt.Id})
		if len(existing) > 0 {
			return renderUpload("upload.error.single_photo_limit")
		}
	}

	files, err := e.FindUploadedFiles("image")
	if err != nil || len(files) == 0 {
		return renderUpload("upload.error.no_file")
	}
	if len(files) > 100 {
		return renderUpload("upload.error.too_many")
	}
	formats := make([]string, len(files))
	var incomingBytes int64
	for i, file := range files {
		if file.Size <= 0 || file.Size > maxUploadFileSize {
			return renderUpload("upload.error.too_large")
		}
		format, _, ok := detectUploadFormat(file)
		if !ok {
			return renderUpload("upload.error.bad_format")
		}
		formats[i] = format
		incomingBytes += file.Size
	}
	if galleryUsageBytes(e.App, eventID)+incomingBytes > galleryStorageLimit {
		return renderUpload("upload.error.gallery_full")
	}

	// A name is required only when the owner turned the setting on. The client
	// also guards this (so the chosen photo isn't lost to a reload), but we
	// enforce it server-side too. Over-long names are trimmed to the schema cap
	// rather than rejected — no reason to block an upload over a stray paste.
	if collectName {
		if guestName == "" {
			return renderUpload("upload.error.name_required")
		}
		if len([]rune(guestName)) > maxGuestNameLen {
			guestName = string([]rune(guestName)[:maxGuestNameLen])
		}
	}

	collection, err := e.App.FindCollectionByNameOrId("uploads")
	if err != nil {
		return e.InternalServerError("Failed to create upload record", err)
	}

	for i, file := range files {
		record := core.NewRecord(collection)
		record.Set("prompt", prompt.Id)
		record.Set("event", eventID)
		record.Set("image", file)
		if collectName {
			record.Set("guest_name", guestName)
		}
		if err := e.App.Save(record); err != nil {
			return e.InternalServerError("Failed to save upload", err)
		}
		// HEIC/HEIF originals stay untouched; only a browser-friendly preview
		// is generated in the background.
		if formats[i] == "heic" || formats[i] == "heif" {
			queueDisplayConversion(e.App, record.Id)
		}
	}

	setSubmissionCookie(e, eventID)

	if qrMode {
		return redirectLocalised(e, http.StatusSeeOther, fmt.Sprintf("/e/%s/done?mode=qr", eventID))
	}
	return redirectLocalised(e, http.StatusSeeOther, fmt.Sprintf("/e/%s/library", eventID))
}

// handleEventLibrary shows guests a thumbnail grid of every prompt with
// contributions, plus a fullscreen swipeable lightbox for the closer-look
// experience. Open access (no auth) so anyone with the QR link can browse
// what's been captured so far.
func handleEventLibrary(e *core.RequestEvent) error {
	eventID := e.Request.PathValue("id")
	if eventID == "" {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}

	event, err := e.App.FindRecordById("events", eventID)
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	if !eventGalleryActive(event) {
		return renderHTMLErrorKeysLang(e, http.StatusGone, guestLang(e, event), "error.title.gallery_expired", "error.message.gallery_expired")
	}

	lang := guestLang(e, event)

	prompts, err := e.App.FindRecordsByFilter("prompts", "event = {:eid}", "sort_order", 1000, 0, dbxParams{"eid": eventID})
	if err != nil {
		return e.InternalServerError("Failed to load prompts", err)
	}

	design := GetDesignByID(event.GetString("design_id"))
	if design == nil {
		design = &Designs[0]
	}

	type libraryItem struct {
		PromptID   string
		PromptText string
		ImageURL   string
		PhotoCount int
		MediaKind  string
	}
	type slide struct {
		ImageURL   string
		PromptText string
		PromptID   string
		GuestName  string
		MediaKind  string
	}

	var items []libraryItem
	var slides []slide
	uploadedCount := 0
	for _, p := range prompts {
		uploads, _ := e.App.FindRecordsByFilter("uploads", "prompt = {:pid}", "-created", 0, 0, dbxParams{"pid": p.Id})
		for _, u := range uploads {
			kind := uploadMediaKind(u)
			items = append(items, libraryItem{PromptID: p.Id, PromptText: p.GetString("text"), ImageURL: uploadDisplayURL(u), PhotoCount: 1, MediaKind: kind})
			slides = append(slides, slide{ImageURL: uploadDisplayURL(u), PromptText: p.GetString("text"), PromptID: p.Id, GuestName: u.GetString("guest_name"), MediaKind: kind})
			uploadedCount++
		}
	}

	// Guest gallery ZIP downloads are a paid feature. Free-tier events
	// never expose the button, regardless of the owner's per-event
	// disable_guest_download toggle.
	guestDownloadEnabled := !disableGuestDownloadEnabled(e.App, event) && eventOwnerPaid(e.App, event)
	already := e.Request.URL.Query().Get("already") == "1"

	return e.HTML(http.StatusOK, renderWithBase(e, "library", map[string]any{
		"GuestPage":            true,
		"Lang":                 lang,
		"Event":                event,
		"Design":               design,
		"Items":                items,
		"Slides":               slides,
		"UploadedCount":        uploadedCount,
		"TotalPrompts":         len(prompts),
		"GuestDownloadEnabled": guestDownloadEnabled,
		"Already":              already,
	}))
}

func handleEventDone(e *core.RequestEvent) error {
	eventID := e.Request.PathValue("id")
	if eventID == "" {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	event, err := e.App.FindRecordById("events", eventID)
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	if !eventGalleryActive(event) {
		return renderHTMLErrorKeysLang(e, http.StatusGone, guestLang(e, event), "error.title.gallery_expired", "error.message.gallery_expired")
	}

	lang := guestLang(e, event)

	firstPromptID := ""
	prompts, _ := e.App.FindRecordsByFilter("prompts", "event = {:eid}", "sort_order", 1, 0, dbxParams{"eid": eventID})
	if len(prompts) > 0 {
		firstPromptID = prompts[0].Id
	}

	locked := lockAfterSubmitEnabled(e.App, event) && guestHasSubmitted(e, eventID)

	// In single-QR mode the "upload another" CTA becomes "get another
	// prompt", routing back through the dispatcher with ?next=1 so the
	// guest is handed a fresh random prompt rather than the first one.
	poster := e.Request.URL.Query().Get("mode") == "qr"

	return e.HTML(http.StatusOK, renderWithBase(e, "done", map[string]any{
		"GuestPage":     true,
		"Lang":          lang,
		"EventID":       eventID,
		"FirstPromptID": firstPromptID,
		"Locked":        locked,
		"Poster":        poster,
	}))
}

// guestCookie is the wire form of the per-browser, per-event progress for
// single-QR mode. Cur is the prompt the guest is currently looking at (sticky
// across refreshes); Seen is a base64-encoded bitset over the event's
// prompts in sort order — bit i set means the prompt at position i has been
// shown. The bitset keeps the cookie tiny even for very large events: 500
// prompts fit in ~63 bytes (vs ~10 KB if we stored 15-char IDs), well within
// the ~4 KB per-cookie browser limit. Persisted in the guest_state_<id>
// cookie, so an incognito window starts blank and gets a fresh prompt.
type guestCookie struct {
	Cur  string `json:"c"`
	Seen string `json:"s"`
}

func guestStateCookieName(eventID string) string {
	return "guest_state_" + eventID
}

// promptIDs returns the prompt IDs in their current sort order; the slice index
// is the bit position used by the seen bitset.
func promptIDs(prompts []*core.Record) []string {
	ids := make([]string, len(prompts))
	for i, p := range prompts {
		ids[i] = p.Id
	}
	return ids
}

// encodeGuestCookie packs the current prompt + seen set into the base64 cookie
// value. orderedIDs fixes the bit positions; only seen IDs present in orderedIDs
// are recorded, so prompts the owner later deletes simply drop out.
func encodeGuestCookie(cur string, orderedIDs []string, seen map[string]bool) string {
	bits := make([]byte, (len(orderedIDs)+7)/8)
	for i, id := range orderedIDs {
		if seen[id] {
			bits[i/8] |= 1 << (uint(i) % 8)
		}
	}
	gc := guestCookie{Cur: cur, Seen: base64.StdEncoding.EncodeToString(bits)}
	raw, err := json.Marshal(gc)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// decodeGuestCookie is the inverse of encodeGuestCookie. Any problem (missing,
// malformed, tampered) yields an empty current + empty seen set, which the
// dispatcher treats as a brand-new guest. The seen set is rebuilt against the
// current prompt order, so bits past the current prompt count are ignored.
func decodeGuestCookie(value string, orderedIDs []string) (cur string, seen map[string]bool) {
	seen = map[string]bool{}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", seen
	}
	var gc guestCookie
	if err := json.Unmarshal(decoded, &gc); err != nil {
		return "", seen
	}
	bits, err := base64.StdEncoding.DecodeString(gc.Seen)
	if err != nil {
		bits = nil
	}
	for i, id := range orderedIDs {
		if i/8 < len(bits) && bits[i/8]&(1<<(uint(i)%8)) != 0 {
			seen[id] = true
		}
	}
	return gc.Cur, seen
}

// readGuestState decodes the guest cookie for an event against the current
// prompt list. Returns the sticky prompt ID (or "") and the set of seen IDs.
func readGuestState(e *core.RequestEvent, eventID string, prompts []*core.Record) (cur string, seen map[string]bool) {
	c, err := e.Request.Cookie(guestStateCookieName(eventID))
	if err != nil || c.Value == "" {
		return "", map[string]bool{}
	}
	return decodeGuestCookie(c.Value, promptIDs(prompts))
}

// writeGuestState persists the guest cookie. Lifetime matches the submission
// cookie (180 days) so the event day plus a long after-party tail is covered.
func writeGuestState(e *core.RequestEvent, eventID string, prompts []*core.Record, cur string, seen map[string]bool) {
	http.SetCookie(e.Response, &http.Cookie{
		Name:     guestStateCookieName(eventID),
		Value:    encodeGuestCookie(cur, promptIDs(prompts), seen),
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 180,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// eventHasUploadSet returns the set of prompt IDs that already have at least
// one upload for the event, built with a single query.
func eventHasUploadSet(app core.App, eventID string) map[string]bool {
	set := map[string]bool{}
	uploads, err := app.FindRecordsByFilter("uploads", "event = {:eid}", "", 0, 0, dbxParams{"eid": eventID})
	if err != nil {
		return set
	}
	for _, u := range uploads {
		set[u.GetString("prompt")] = true
	}
	return set
}

// incrementPromptShowCount bumps a prompt's global show counter atomically.
// It's only a tie-breaker for selection, so a best-effort raw UPDATE (which
// skips the record save hooks and the `updated` autodate) is exactly right and
// avoids a read-modify-write race between concurrent guests.
func incrementPromptShowCount(app core.App, promptID string) {
	_, _ = app.DB().
		NewQuery("UPDATE prompts SET show_count = show_count + 1 WHERE id = {:id}").
		Bind(dbxParams{"id": promptID}).
		Execute()
}

// handleEventDispatch is the landing target for the single printed QR
// poster (app_url/e/{id}). It hands the guest one random prompt, keeps it
// sticky across refreshes via the guest cookie, and advances to a new one when
// asked (?next=1). Unauthenticated by design, like the per-prompt upload flow.
func handleEventDispatch(e *core.RequestEvent) error {
	eventID := e.Request.PathValue("id")
	if eventID == "" {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}

	event, err := e.App.FindRecordById("events", eventID)
	if err != nil {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.gallery_not_found")
	}
	if !eventGalleryActive(event) {
		return renderHTMLErrorKeysLang(e, http.StatusGone, guestLang(e, event), "error.title.gallery_expired", "error.message.gallery_expired")
	}

	// Single-QR is opt-in per event. If it's off — including the case where
	// the owner turned it off after printing a poster — send scanners to the
	// public library so a stale QR still lands somewhere useful instead of 404.
	if !event.GetBool("single_qr_mode") {
		return redirectLocalised(e, http.StatusSeeOther, fmt.Sprintf("/e/%s/library", eventID))
	}

	prompts, err := e.App.FindRecordsByFilter("prompts", "event = {:eid}", "sort_order", 1000, 0, dbxParams{"eid": eventID})
	if err != nil || len(prompts) == 0 {
		return renderHTMLErrorKeysLang(e, http.StatusNotFound, guestLang(e, event), "error.title.not_found", "error.message.upload_destination_not_found")
	}

	// Same one-upload-per-guest lock as the per-prompt flow; owners previewing
	// their own event (authenticated) bypass it.
	if lockAfterSubmitEnabled(e.App, event) && guestHasSubmitted(e, eventID) && (e.Auth == nil || e.Auth.Id != event.GetString("owner")) {
		return redirectLocalised(e, http.StatusSeeOther, fmt.Sprintf("/e/%s/library?already=1", eventID))
	}

	byID := make(map[string]*core.Record, len(prompts))
	for _, p := range prompts {
		byID[p.Id] = p
	}

	// Decode the cookie against the current prompt list. Seen is rebuilt from
	// prompt positions, so prompts the owner deleted drop out automatically;
	// clear the current prompt too if it no longer exists.
	cur, seen := readGuestState(e, eventID, prompts)
	if _, ok := byID[cur]; !ok {
		cur = ""
	}

	// Explicit "get another prompt": retire the current prompt, then re-pick.
	if e.Request.URL.Query().Get("next") == "1" && cur != "" {
		seen[cur] = true
		cur = ""
	}

	if cur == "" {
		chosen := pickPrompt(prompts, seen, eventHasUploadSet(e.App, eventID))
		if chosen == nil {
			return renderHTMLErrorKeysLang(e, http.StatusNotFound, guestLang(e, event), "error.title.not_found", "error.message.upload_destination_not_found")
		}
		cur = chosen.Id
		incrementPromptShowCount(e.App, chosen.Id)
	}

	writeGuestState(e, eventID, prompts, cur, seen)

	prompt := byID[cur]
	design := GetDesignByID(event.GetString("design_id"))
	if design == nil {
		design = &Designs[0]
	}

	return e.HTML(http.StatusOK, renderWithBase(e, "challenge", map[string]any{
		"GuestPage":   true,
		"Lang":        guestLang(e, event),
		"Event":       event,
		"Design":      design,
		"Prompt":      prompt,
		"PromptID":    prompt.Id,
		"PromptText":  prompt.GetString("text"),
		"SingleQR":    true,
		"CollectName": collectGuestNameEnabled(e.App, event),
	}))
}

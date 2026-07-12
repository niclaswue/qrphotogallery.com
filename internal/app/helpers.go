package app

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// isValidDate returns true if s parses as an ISO-8601 YYYY-MM-DD calendar
// date. Callers are expected to short-circuit on empty input.
func isValidDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

type Tier struct {
	Name       string
	MaxPrompts int
}

func getUserTier(user *core.Record) Tier {
	tierName := "free"
	if user != nil {
		tierName = user.GetString("tier")
		if tierName == "" {
			tierName = "free"
		}
	}

	for _, t := range appConfig.Tiers {
		if t.Name == tierName {
			return Tier{Name: t.Name, MaxPrompts: t.MaxPrompts}
		}
	}
	return Tier{Name: "free", MaxPrompts: appConfig.Tiers[0].MaxPrompts}
}

// isPaidTier reports whether name is one of the paid plans (anything other
// than the free tier). The paid guest features — multiple uploads per
// prompt and the guest gallery ZIP download — are gated on this.
func isPaidTier(name string) bool {
	return name != "" && name != "free"
}

func isPremiumTier(name string) bool { return name == "premium" }

// eventOwnerPaid reports whether the owner of event is on a paid plan.
// Guest-facing upload/download routes are unauthenticated, so the owner's
// tier can't come from e.Auth — it's resolved from the event's owner record.
// A missing owner (deleted account) counts as free.
//
// It gates the paid guest features: multiple uploads per prompt, the guest
// gallery ZIP, the lock-after-submit toggle and the collect-name toggle.
// Because the per-event toggles only take effect while the owner is paid, a
// downgrade quietly disables them without touching stored settings.
func eventOwnerPaid(app core.App, event *core.Record) bool {
	owner, err := app.FindRecordById("users", event.GetString("owner"))
	if err != nil || owner == nil {
		return false
	}
	return isPaidTier(getUserTier(owner).Name)
}

func eventOwnerPremium(app core.App, event *core.Record) bool {
	owner, err := app.FindRecordById("users", event.GetString("owner"))
	return err == nil && owner != nil && isPremiumTier(getUserTier(owner).Name)
}

// lockAfterSubmitEnabled reports whether the one-upload-per-guest lock is
// active for an event. Like the guest ZIP download, it's a paid feature:
// the per-event toggle only takes effect while the owner is on a paid
// plan, so a downgrade quietly disables it without touching stored settings.
func lockAfterSubmitEnabled(app core.App, event *core.Record) bool {
	return event.GetBool("lock_after_submit") && eventOwnerPremium(app, event)
}

// collectGuestNameEnabled reports whether the guest upload form should ask for
// (and require) a name for an event. Like the one-upload lock and the guest
// ZIP download it's a paid feature: the per-event toggle only takes effect
// while the owner is on a paid plan, so a downgrade quietly disables it
// without touching stored settings.
func collectGuestNameEnabled(app core.App, event *core.Record) bool {
	return event.GetBool("collect_guest_name") && eventOwnerPremium(app, event)
}

func disableGuestDownloadEnabled(app core.App, event *core.Record) bool {
	return event.GetBool("disable_guest_download") && eventOwnerPremium(app, event)
}

// maxGuestNameLen caps a submitted guest name. Matches the uploads.guest_name
// schema limit; enforced server-side so the field is safe even if the client
// strips the maxlength attribute.
const maxGuestNameLen = 60

// safeRedirect returns raw if it is a safe same-origin path, otherwise fallback.
// A safe path starts with a single "/" and does not begin with "//" or "/\",
// which browsers may interpret as a protocol-relative URL or open redirect.
func safeRedirect(raw, fallback string) string {
	if raw == "" {
		return fallback
	}
	if strings.ContainsAny(raw, "\r\n") {
		return fallback
	}
	if !strings.HasPrefix(raw, "/") {
		return fallback
	}
	if strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") {
		return fallback
	}
	return raw
}

// redirectToRegister sends an unauthenticated visitor to the sign-up page,
// remembering the full URL they were trying to reach (path + query) so they
// land back there the instant they finish signing up — e.g. straight into
// checkout when they clicked a "buy" link while logged out. handleRegisterSubmit
// / handleLoginSubmit decode the ?redirect= value and safeRedirect there once
// auth succeeds.
//
// The target is URL-encoded before being placed in ?redirect= so URLs with
// more than one parameter survive intact. Without encoding, a link such as
// /payment?plan=premium&variant=higher has its &variant= reparsed as a
// separate parameter of the /register URL and silently dropped — sending the
// new customer to checkout for the wrong (default) price, a likely reason to
// abandon and "forget" to pay.
func redirectToRegister(e *core.RequestEvent) error {
	target := url.QueryEscape(e.Request.URL.RequestURI())
	return redirectLocalised(e, http.StatusSeeOther, "/register?redirect="+target)
}

// detectImageFormat sniffs the first bytes of an upload and returns a short
// canonical name for the format ("jpeg", "png", "gif", "webp", "heic",
// "heif", "bmp", "tiff") plus ok=true. Unknown formats return ("", false).
//
// Coverage is intentionally broad: phones and cameras emit a wide range of
// image containers (HEIC from modern iPhones, BMP/TIFF from some Android
// galleries and DSLRs) and a strict whitelist rejects legitimate guest
// uploads at the door. HEIC/HEIF are detected as a distinct format so
// the upload handler can store the original and queue a server-side JPEG
// transcode for browser display (see imageconv.go).
func detectImageFormat(f *filesystem.File) (string, bool) {
	if f.Reader == nil {
		return "", false
	}
	buf := make([]byte, 16)
	r, err := f.Reader.Open()
	if err != nil {
		return "", false
	}
	defer r.Close()
	n, err := r.Read(buf)
	if err != nil || n < 4 {
		return "", false
	}
	head := buf[:n]

	// JPEG: FF D8 FF
	if n >= 3 && bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}) {
		return "jpeg", true
	}
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if n >= 8 && bytes.HasPrefix(head, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "png", true
	}
	// GIF87a / GIF89a
	if n >= 6 && (bytes.HasPrefix(head, []byte("GIF87a")) || bytes.HasPrefix(head, []byte("GIF89a"))) {
		return "gif", true
	}
	// WebP: RIFF....WEBP
	if n >= 12 && bytes.HasPrefix(head, []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")) {
		return "webp", true
	}
	// BMP: 'BM'
	if n >= 2 && head[0] == 'B' && head[1] == 'M' {
		return "bmp", true
	}
	// TIFF: II*\0 (little-endian) or MM\0* (big-endian).
	if n >= 4 && (bytes.Equal(head[:4], []byte{'I', 'I', 0x2A, 0x00}) || bytes.Equal(head[:4], []byte{'M', 'M', 0x00, 0x2A})) {
		return "tiff", true
	}
	// HEIC / HEIF: ISO base media file with ftyp box at offset 4.
	// Layout: [4 bytes box size][ftyp][4 bytes major brand][...]
	if n >= 12 && bytes.Equal(head[4:8], []byte("ftyp")) {
		brand := string(head[8:12])
		switch brand {
		case "heic", "heix", "heim", "heis":
			return "heic", true
		case "hevc", "hevx", "hevm", "hevs":
			return "heic", true
		case "mif1", "msf1", "avif", "avis":
			return "heif", true
		}
	}
	return "", false
}

// detectUploadFormat accepts both full-resolution stills and the video
// containers guests commonly produce on phones and cameras. Every decision
// is based on file contents (with the original filename used only to
// distinguish compatible ISO media containers), so renaming an arbitrary
// executable never makes it uploadable.
func detectUploadFormat(f *filesystem.File) (format, kind string, ok bool) {
	if format, ok := detectImageFormat(f); ok {
		return format, "image", true
	}
	if f == nil || f.Reader == nil {
		return "", "", false
	}
	r, err := f.Reader.Open()
	if err != nil {
		return "", "", false
	}
	defer r.Close()
	buf := make([]byte, 32)
	n, _ := r.Read(buf)
	head := buf[:n]
	ext := strings.ToLower(f.OriginalName)
	if n >= 12 && bytes.Equal(head[4:8], []byte("ftyp")) {
		brand := string(head[8:12])
		switch brand {
		case "qt  ":
			return "mov", "video", true
		case "3gp4", "3gp5", "3gp6", "3g2a", "3g2b":
			return "3gp", "video", true
		default:
			if strings.HasSuffix(ext, ".mp4") || strings.HasSuffix(ext, ".m4v") || strings.HasSuffix(ext, ".mov") {
				return "mp4", "video", true
			}
		}
	}
	if n >= 4 && bytes.Equal(head[:4], []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		return "webm", "video", true
	}
	if n >= 12 && bytes.HasPrefix(head, []byte("RIFF")) && bytes.Equal(head[8:12], []byte("AVI ")) {
		return "avi", "video", true
	}
	if n >= 4 && (bytes.HasPrefix(head, []byte{0x00, 0x00, 0x01, 0xBA}) || bytes.HasPrefix(head, []byte{0x00, 0x00, 0x01, 0xB3})) {
		return "mpeg", "video", true
	}
	return "", "", false
}

func uploadMediaKind(u *core.Record) string {
	name := strings.ToLower(u.GetString("image"))
	for _, ext := range []string{".mp4", ".mov", ".m4v", ".webm", ".avi", ".mpeg", ".mpg", ".3gp"} {
		if strings.HasSuffix(name, ext) {
			return "video"
		}
	}
	return "image"
}

const (
	galleryStorageLimit  = int64(100 * 1024 * 1024 * 1024)
	maxUploadFileSize    = int64(2 * 1024 * 1024 * 1024)
	maxUploadRequestSize = int64(4 * 1024 * 1024 * 1024)
)

func galleryUsageBytes(app core.App, eventID string) int64 {
	uploads, err := app.FindRecordsByFilter("uploads", "event = {:eid}", "", 0, 0, dbxParams{"eid": eventID})
	if err != nil {
		return 0
	}
	fsys, err := app.NewFilesystem()
	if err != nil {
		return 0
	}
	defer fsys.Close()
	var total int64
	for _, u := range uploads {
		if name := u.GetString("image"); name != "" {
			if attrs, err := fsys.Attributes(u.BaseFilesPath() + "/" + name); err == nil {
				total += attrs.Size
			}
		}
	}
	return total
}

func eventGalleryActive(event *core.Record) bool {
	created := event.GetDateTime("created").Time()
	return created.IsZero() || time.Now().Before(created.AddDate(1, 0, 0))
}

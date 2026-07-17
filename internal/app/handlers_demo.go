package app

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/types"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

const (
	demoGalleryLifetime = time.Hour
	demoMaxImageSize    = int64(15 << 20)
	demoMaxRequestSize  = int64(17 << 20)
	demoSessionCookie   = "qrpg_demo_gallery"
	demoSamplePath      = "pb_public/static/img/gallery-sample/guests-toast.webp"
)

type demoState struct {
	Stage       string `json:"stage"`
	ImageURL    string `json:"image_url,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	Version     int64  `json:"version"`
}

// handleCreateDemoGallery creates the anonymous gallery behind the hero QR.
// A short-lived HttpOnly cookie lets a visitor reload the landing page without
// creating another record, while a different browser still receives a unique
// gallery.
func handleCreateDemoGallery(e *core.RequestEvent) error {
	if !demoSameOriginRequest(e) {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}

	lang := e.Request.FormValue("lang")
	if !i18n.IsSupported(lang) {
		lang = i18n.DefaultLang
	}

	if cookie, err := e.Request.Cookie(demoSessionCookie); err == nil && cookie.Value != "" {
		if record, err := e.App.FindRecordById("demo_galleries", cookie.Value); err == nil && demoGalleryActive(record) {
			return writeDemoSession(e, record, lang)
		}
	}

	collection, err := e.App.FindCollectionByNameOrId("demo_galleries")
	if err != nil {
		return e.InternalServerError("Failed to start demo gallery", err)
	}
	record := core.NewRecord(collection)
	record.Set("lang", lang)
	record.Set("expires_at", types.NowDateTime().Add(demoGalleryLifetime))
	if err := e.App.Save(record); err != nil {
		return e.InternalServerError("Failed to start demo gallery", err)
	}

	return writeDemoSession(e, record, lang)
}

func writeDemoSession(e *core.RequestEvent, record *core.Record, lang string) error {
	if record.GetString("lang") != lang {
		record.Set("lang", lang)
		_ = e.App.Save(record)
	}

	http.SetCookie(e.Response, &http.Cookie{
		Name:     demoSessionCookie,
		Value:    record.Id,
		Path:     "/",
		MaxAge:   int(demoGalleryLifetime.Seconds()),
		HttpOnly: true,
		Secure:   strings.HasPrefix(strings.ToLower(appConfig.AppURL), "https://"),
		SameSite: http.SameSiteLaxMode,
	})
	demoNoStore(e)
	return e.JSON(http.StatusCreated, map[string]any{
		"id":         record.Id,
		"demo_url":   demoPagePath(record, lang),
		"qr_url":     fmt.Sprintf("/demo/%s/qr.png?lang=%s", record.Id, url.QueryEscape(lang)),
		"state_url":  fmt.Sprintf("/demo/%s/state", record.Id),
		"expires_at": record.GetDateTime("expires_at"),
	})
}

// handleDemoGallery is intentionally rendered without the normal application
// shell. A QR scan should reach the picker quickly, without navigation,
// analytics, cookie banners, or unrelated JavaScript.
func handleDemoGallery(e *core.RequestEvent) error {
	demoNoStore(e)
	demoPageSecurityHeaders(e)

	record, err := e.App.FindRecordById("demo_galleries", e.Request.PathValue("id"))
	if err != nil {
		lang := demoRequestedLang(e, nil)
		return e.HTML(http.StatusNotFound, renderStandalone("demo", lang, map[string]any{
			"NotFound": true,
		}))
	}
	lang := demoRequestedLang(e, record)
	if !demoGalleryActive(record) {
		return e.HTML(http.StatusGone, renderStandalone("demo", lang, map[string]any{
			"Expired": true,
		}))
	}

	if record.GetDateTime("scanned_at").IsZero() {
		record.Set("scanned_at", types.NowDateTime())
		if err := e.App.Save(record); err != nil {
			return e.InternalServerError("Failed to open demo gallery", err)
		}
	}

	state := buildDemoState(record)
	return e.HTML(http.StatusOK, renderStandalone("demo", lang, map[string]any{
		"SessionID":   record.Id,
		"HasPhoto":    state.Stage == "photo",
		"ImageURL":    state.ImageURL,
		"DownloadURL": state.DownloadURL,
		"UploadURL":   fmt.Sprintf("/demo/%s/upload", record.Id),
		"SampleURL":   fmt.Sprintf("/demo/%s/sample", record.Id),
		"ExpiresAt":   record.GetDateTime("expires_at").Time().UTC().Format(time.RFC3339),
	}))
}

func handleDemoGalleryState(e *core.RequestEvent) error {
	demoNoStore(e)
	record, err := e.App.FindRecordById("demo_galleries", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
	}
	if !demoGalleryActive(record) {
		return e.JSON(http.StatusGone, map[string]string{"error": "expired"})
	}
	return e.JSON(http.StatusOK, buildDemoState(record))
}

func handleDemoGalleryQR(e *core.RequestEvent) error {
	demoNoStore(e)
	record, err := activeDemoGallery(e)
	if err != nil {
		return e.JSON(http.StatusGone, map[string]string{"error": "expired"})
	}
	lang := demoRequestedLang(e, record)
	publicURL := strings.TrimRight(appConfig.AppURL, "/") + demoPagePath(record, lang)
	png, err := qrcode.Encode(publicURL, qrcode.Medium, 420)
	if err != nil {
		return e.InternalServerError("Failed to render demo QR", err)
	}
	e.Response.Header().Set("X-Content-Type-Options", "nosniff")
	return e.Blob(http.StatusOK, "image/png", png)
}

func handleDemoGalleryUpload(e *core.RequestEvent) error {
	demoNoStore(e)
	if !demoSameOriginRequest(e) {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}
	record, err := activeDemoGallery(e)
	if err != nil {
		return demoJSONError(e, http.StatusGone, nil, "demo.error.expired")
	}
	if demoGalleryHasPhoto(record) {
		return demoJSONError(e, http.StatusConflict, record, "demo.error.already_uploaded")
	}

	files, err := e.FindUploadedFiles("image")
	if err != nil || len(files) != 1 {
		return demoJSONError(e, http.StatusBadRequest, record, "demo.error.no_file")
	}
	file := files[0]
	if file.Size <= 0 || file.Size > demoMaxImageSize {
		return demoJSONError(e, http.StatusRequestEntityTooLarge, record, "demo.error.too_large")
	}
	format, ok := detectImageFormat(file)
	if !ok || !demoDisplayFormat(format) {
		return demoJSONError(e, http.StatusUnsupportedMediaType, record, "demo.error.bad_format")
	}

	record.Set("image", file)
	record.Set("format", format)
	record.Set("original_name", demoOriginalFilename(file.OriginalName, format))

	// HEIC/HEIF is common on iPhones but not consistently renderable in web
	// browsers. Convert this single, bounded demo upload before responding so
	// the desktop gallery can display it on its very next poll.
	if format == "heic" || format == "heif" {
		select {
		case heicConvertSem <- struct{}{}:
			defer func() { <-heicConvertSem }()
		case <-e.Request.Context().Done():
			return demoJSONError(e, http.StatusRequestTimeout, record, "demo.error.failed")
		}
		reader, openErr := file.Reader.Open()
		if openErr != nil {
			return demoJSONError(e, http.StatusUnsupportedMediaType, record, "demo.error.bad_format")
		}
		jpegBytes, convertErr := heicToJPEG(reader)
		reader.Close()
		if convertErr != nil {
			return demoJSONError(e, http.StatusUnsupportedMediaType, record, "demo.error.bad_format")
		}
		display, fileErr := filesystem.NewFileFromBytes(jpegBytes, displayFilename(file.OriginalName))
		if fileErr != nil {
			return demoJSONError(e, http.StatusInternalServerError, record, "demo.error.failed")
		}
		record.Set("display", display)
	}

	if err := e.App.Save(record); err != nil {
		return demoJSONError(e, http.StatusInternalServerError, record, "demo.error.failed")
	}
	return e.JSON(http.StatusCreated, buildDemoState(record))
}

func handleDemoGallerySample(e *core.RequestEvent) error {
	demoNoStore(e)
	if !demoSameOriginRequest(e) {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}
	record, err := activeDemoGallery(e)
	if err != nil {
		return demoJSONError(e, http.StatusGone, nil, "demo.error.expired")
	}
	if demoGalleryHasPhoto(record) {
		return demoJSONError(e, http.StatusConflict, record, "demo.error.already_uploaded")
	}
	record.Set("sample", true)
	record.Set("format", "webp")
	record.Set("original_name", "demo-photo.webp")
	if err := e.App.Save(record); err != nil {
		return demoJSONError(e, http.StatusInternalServerError, record, "demo.error.failed")
	}
	return e.JSON(http.StatusCreated, buildDemoState(record))
}

func handleDemoGalleryImage(e *core.RequestEvent) error {
	return serveDemoGalleryImage(e, false)
}

func handleDemoGalleryDownload(e *core.RequestEvent) error {
	return serveDemoGalleryImage(e, true)
}

func serveDemoGalleryImage(e *core.RequestEvent, download bool) error {
	demoNoStore(e)
	record, err := activeDemoGallery(e)
	if err != nil || !demoGalleryHasPhoto(record) {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
	}
	e.Response.Header().Set("X-Content-Type-Options", "nosniff")

	filename := record.GetString("original_name")
	if filename == "" {
		filename = "demo-photo"
	}
	if download {
		e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeFilename(filename)))
	} else {
		e.Response.Header().Set("Content-Disposition", "inline")
	}

	if record.GetBool("sample") {
		file, err := os.Open(demoSamplePath)
		if err != nil {
			return e.InternalServerError("Failed to open demo image", err)
		}
		defer file.Close()
		if info, statErr := file.Stat(); statErr == nil {
			e.Response.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		}
		return e.Stream(http.StatusOK, "image/webp", file)
	}

	field := "image"
	contentType := demoContentType(record.GetString("format"))
	if !download && record.GetString("display") != "" {
		field = "display"
		contentType = "image/jpeg"
	}
	storedName := record.GetString(field)
	if storedName == "" {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
	}
	fsys, err := e.App.NewFilesystem()
	if err != nil {
		return e.InternalServerError("Failed to open demo storage", err)
	}
	defer fsys.Close()
	key := record.BaseFilesPath() + "/" + storedName
	if attrs, attrErr := fsys.Attributes(key); attrErr == nil {
		e.Response.Header().Set("Content-Length", strconv.FormatInt(attrs.Size, 10))
	}
	reader, err := fsys.GetReader(key)
	if err != nil {
		return e.InternalServerError("Failed to open demo image", err)
	}
	defer reader.Close()
	return e.Stream(http.StatusOK, contentType, reader)
}

func activeDemoGallery(e *core.RequestEvent) (*core.Record, error) {
	record, err := e.App.FindRecordById("demo_galleries", e.Request.PathValue("id"))
	if err != nil || !demoGalleryActive(record) {
		return nil, fmt.Errorf("demo gallery not found or expired")
	}
	return record, nil
}

func demoGalleryActive(record *core.Record) bool {
	if record == nil {
		return false
	}
	expires := record.GetDateTime("expires_at")
	return !expires.IsZero() && time.Now().Before(expires.Time())
}

func demoGalleryHasPhoto(record *core.Record) bool {
	return record != nil && (record.GetBool("sample") || record.GetString("image") != "")
}

func buildDemoState(record *core.Record) demoState {
	stage := "ready"
	if !record.GetDateTime("scanned_at").IsZero() {
		stage = "scanned"
	}
	state := demoState{Stage: stage, Version: record.GetDateTime("updated").Unix()}
	if demoGalleryHasPhoto(record) {
		state.Stage = "photo"
		state.ImageURL = fmt.Sprintf("/demo/%s/image?v=%d", record.Id, state.Version)
		state.DownloadURL = fmt.Sprintf("/demo/%s/download", record.Id)
	}
	return state
}

func demoPagePath(record *core.Record, lang string) string {
	return fmt.Sprintf("/demo/%s?lang=%s", record.Id, url.QueryEscape(lang))
}

func demoRequestedLang(e *core.RequestEvent, record *core.Record) string {
	lang := e.Request.URL.Query().Get("lang")
	if !i18n.IsSupported(lang) && record != nil {
		lang = record.GetString("lang")
	}
	if !i18n.IsSupported(lang) {
		lang = i18n.DefaultLang
	}
	return lang
}

func demoJSONError(e *core.RequestEvent, status int, record *core.Record, key string) error {
	lang := demoRequestedLang(e, record)
	return e.JSON(status, map[string]string{"error": i18n.T(lang, key)})
}

func demoNoStore(e *core.RequestEvent) {
	e.Response.Header().Set("Cache-Control", "private, no-store")
	e.Response.Header().Set("Pragma", "no-cache")
}

func demoPageSecurityHeaders(e *core.RequestEvent) {
	e.Response.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	e.Response.Header().Set("Referrer-Policy", "no-referrer")
	e.Response.Header().Set("X-Content-Type-Options", "nosniff")
}

func demoSameOriginRequest(e *core.RequestEvent) bool {
	origin := strings.TrimSpace(e.Request.Header.Get("Origin"))
	if origin != "" {
		parsed, err := url.Parse(origin)
		return err == nil && strings.EqualFold(parsed.Host, e.Request.Host)
	}
	return e.Request.Header.Get("Sec-Fetch-Site") != "cross-site"
}

func demoDisplayFormat(format string) bool {
	switch format {
	case "jpeg", "png", "gif", "webp", "heic", "heif":
		return true
	default:
		return false
	}
}

func demoContentType(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "heic":
		return "image/heic"
	case "heif":
		return "image/heif"
	default:
		return "application/octet-stream"
	}
}

func demoOriginalFilename(original, format string) string {
	name := sanitizeFilename(filepath.Base(original))
	ext := strings.ToLower(filepath.Ext(name))
	validExt := map[string][]string{
		"jpeg": {".jpg", ".jpeg"},
		"png":  {".png"},
		"gif":  {".gif"},
		"webp": {".webp"},
		"heic": {".heic"},
		"heif": {".heif"},
	}
	valid := false
	for _, allowed := range validExt[format] {
		if ext == allowed {
			valid = true
			break
		}
	}
	if !valid {
		ext = "." + format
		if format == "jpeg" {
			ext = ".jpg"
		}
		name = "demo-photo" + ext
	}
	runes := []rune(name)
	if len(runes) > 180 {
		name = string(runes[:180-len([]rune(ext))]) + ext
	}
	return name
}

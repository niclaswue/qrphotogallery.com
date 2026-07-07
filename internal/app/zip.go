package app

import (
	"archive/zip"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// sanitizeFilename returns a safe ASCII filename suitable for a
// Content-Disposition header. Characters that would break header parsing
// (CR, LF, quote, backslash, control chars) are replaced with '_'.
// Non-ASCII is stripped to keep the header valid without RFC 5987 encoding.
func sanitizeFilename(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r < 0x20, r == 0x7f:
			b.WriteByte('_')
		case r == '"', r == '\\', r == '/':
			b.WriteByte('_')
		case r > 0x7e:
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		out = "file"
	}
	return out
}

func downloadGalleryZip(e *core.RequestEvent, event *core.Record) error {
	uploads, err := e.App.FindRecordsByFilter(
		"uploads",
		"event = {:eid}",
		"-created",
		10000,
		0,
		dbxParams{"eid": event.Id},
	)
	if err != nil {
		return e.InternalServerError("Failed to load uploads", err)
	}

	// Read photos through PocketBase's configured filesystem (local *or* S3)
	// rather than os.Open on a hardcoded local disk path. With S3 storage
	// enabled the bytes never touch local disk, so the old direct-disk read
	// failed for every file and silently shipped an empty 22-byte archive.
	// Opening the filesystem before we commit the ZIP response headers also lets
	// a misconfigured backend surface as a clean 500 instead of a broken file.
	fsys, err := e.App.NewFilesystem()
	if err != nil {
		return e.InternalServerError("Failed to open file storage", err)
	}
	defer fsys.Close()

	e.Response.Header().Set("Content-Type", "application/zip")
	e.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-gallery.zip\"", sanitizeFilename(event.GetString("title"))))

	written, err := writeGalleryZip(e.App, fsys, e.Response, uploads)
	if err != nil {
		// The ZIP body is already streaming, so we can't switch to a 500 here;
		// log so a finalise failure is diagnosable instead of silent.
		e.App.Logger().Error("gallery zip: failed to finalise archive", "event", event.Id, "error", err)
	}
	// An event with uploads that yields zero archive entries means none of
	// the bytes were readable — almost always a storage backend problem. Log it
	// loudly so this can't regress back into a silent empty download.
	if written == 0 && len(uploads) > 0 {
		e.App.Logger().Error("gallery zip: empty archive despite uploads present — check file storage backend", "event", event.Id, "uploads", len(uploads))
	}
	return nil
}

// writeGalleryZip streams every upload's stored photo into w as a ZIP, reading
// each file from fsys (the app's configured storage backend) so it works with
// both local and S3 storage. It returns the number of entries written; a single
// unreadable file is logged and skipped rather than aborting the whole archive.
func writeGalleryZip(app core.App, fsys *filesystem.System, w io.Writer, uploads []*core.Record) (int, error) {
	zipWriter := zip.NewWriter(w)

	usedNames := map[string]int{}
	written := 0
	for _, upload := range uploads {
		// Prefer the browser-friendly JPEG rendition (HEIC uploads) so the
		// downloaded archive is viewable everywhere; fall back to the original.
		filename := upload.GetString("display")
		if filename == "" {
			filename = upload.GetString("image")
		}
		if filename == "" {
			continue
		}

		key := upload.BaseFilesPath() + "/" + filename
		reader, err := fsys.GetReader(key)
		if err != nil {
			app.Logger().Error("gallery zip: cannot open upload file", "upload", upload.Id, "key", key, "error", err)
			continue
		}

		prompt, _ := app.FindRecordById("prompts", upload.GetString("prompt"))
		promptText := "unknown"
		if prompt != nil {
			promptText = prompt.GetString("sort_order") + "-" + prompt.GetString("text")
			if len(promptText) > 50 {
				promptText = promptText[:50]
			}
		}
		// When the host collected names, fold the submitter into the filename so
		// the contest winner is identifiable straight from the archive listing.
		label := promptText
		if gn := strings.TrimSpace(upload.GetString("guest_name")); gn != "" {
			label = promptText + " - " + zipEntryName(gn)
		}
		ext := filepath.Ext(filename)
		baseName := fmt.Sprintf("%s%s", label, ext)
		zipName := baseName
		if n := usedNames[baseName]; n > 0 {
			zipName = fmt.Sprintf("%s-%d%s", label, n+1, ext)
		}
		usedNames[baseName]++

		entry, err := zipWriter.Create(zipName)
		if err != nil {
			reader.Close()
			app.Logger().Error("gallery zip: cannot create archive entry", "name", zipName, "error", err)
			continue
		}
		_, err = io.Copy(entry, reader)
		reader.Close()
		if err != nil {
			app.Logger().Error("gallery zip: cannot stream upload into archive", "upload", upload.Id, "error", err)
			continue
		}
		written++
	}

	if err := zipWriter.Close(); err != nil {
		return written, err
	}
	return written, nil
}
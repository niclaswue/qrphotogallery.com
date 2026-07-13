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
	for index, upload := range uploads {
		// Export the original file exactly as it was uploaded. The optional
		// display rendition exists only to make HEIC photos viewable in browsers.
		filename := upload.GetString("image")
		if filename == "" {
			filename = upload.GetString("display")
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

		// The archive is a flat gallery, ordered exactly like the upload query.
		// Keep the stored original filename and add a numeric prefix so files sort
		// predictably without exposing the hidden upload bucket.
		originalBase := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		label := fmt.Sprintf("%04d-%s", index+1, safeZipLabel(originalBase))
		if gn := strings.TrimSpace(upload.GetString("guest_name")); gn != "" {
			label = fmt.Sprintf("%04d-%s-%s", index+1, safeZipLabel(gn), safeZipLabel(originalBase))
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

func safeZipLabel(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		out = "upload"
	}
	if runes := []rune(out); len(runes) > 80 {
		out = string(runes[:80])
	}
	return out
}

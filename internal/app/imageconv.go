package app

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"strings"

	"github.com/gen2brain/heic"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// heicConvertSem bounds how many HEIC transcodes run at once. Each decode
// spins up a WASM runtime and holds a full-resolution frame in memory, so we
// cap concurrency to keep a burst of iPhone uploads from spiking memory. Two
// is plenty for a single-box event app; uploads still succeed immediately,
// only the background preview waits its turn.
var heicConvertSem = make(chan struct{}, 2)

// heicToJPEG decodes a HEIC/HEIF image and re-encodes it as JPEG. It's a pure
// function (no I/O beyond the supplied reader) so it can be unit-tested with a
// sample file. Quality 88 keeps the gallery crisp without bloating storage.
func heicToJPEG(r io.Reader) ([]byte, error) {
	img, err := heic.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode heic: %w", err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// displayFilename swaps an upload's original filename extension for .jpg so the
// generated rendition downloads/serves as a JPEG (e.g. "photo.heic" ->
// "photo.jpg").
func displayFilename(original string) string {
	if i := strings.LastIndexByte(original, '.'); i > 0 {
		original = original[:i]
	}
	if original == "" {
		original = "photo"
	}
	return original + ".jpg"
}

// queueDisplayConversion kicks off the background HEIC -> JPEG transcode for an
// upload record. The guest's request has already returned by the time this
// runs; we re-open the stored original from the app filesystem rather than
// relying on the request's (consumed) multipart reader. Failures are logged
// and left as-is — the original is still stored, the gallery just can't render
// it inline.
func queueDisplayConversion(app core.App, recordID string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("heic conversion panicked for upload %s: %v", recordID, r)
			}
		}()
		heicConvertSem <- struct{}{}
		defer func() { <-heicConvertSem }()

		if err := convertUploadToDisplay(app, recordID); err != nil {
			log.Printf("heic conversion failed for upload %s: %v", recordID, err)
		}
	}()
}

// convertUploadToDisplay reads an upload's original image from storage, decodes
// it as HEIC, and stores a JPEG rendition in the record's `display` field. It's
// idempotent: a record that already has a display file is left untouched.
func convertUploadToDisplay(app core.App, recordID string) error {
	record, err := app.FindRecordById("uploads", recordID)
	if err != nil {
		return err
	}
	if record.GetString("display") != "" {
		return nil // already converted
	}
	original := record.GetString("image")
	if original == "" {
		return fmt.Errorf("upload has no image")
	}

	fsys, err := app.NewFilesystem()
	if err != nil {
		return err
	}
	defer fsys.Close()

	reader, err := fsys.GetReader(record.BaseFilesPath() + "/" + original)
	if err != nil {
		return err
	}
	defer reader.Close()

	jpegBytes, err := heicToJPEG(reader)
	if err != nil {
		return err
	}

	file, err := filesystem.NewFileFromBytes(jpegBytes, displayFilename(original))
	if err != nil {
		return err
	}
	record.Set("display", file)
	return app.Save(record)
}

// uploadDisplayURL returns the path to the browser-renderable image for an
// upload: the generated JPEG rendition when present (HEIC uploads), otherwise
// the original file. Callers prefix the app origin / use it as a relative URL.
func uploadDisplayURL(u *core.Record) string {
	if d := u.GetString("display"); d != "" {
		return u.BaseFilesPath() + "/" + d
	}
	return u.BaseFilesPath() + "/" + u.GetString("image")
}

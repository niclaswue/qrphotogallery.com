package app

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// readGalleryZip drives downloadGalleryZip against a recorder and parses the
// streamed archive into a name->bytes map.
func readGalleryZip(t *testing.T, app core.App, event *core.Record) (map[string][]byte, int) {
	t.Helper()

	e := &core.RequestEvent{}
	e.App = app
	e.Request = httptest.NewRequest(http.MethodGet, "/download/"+event.Id, nil)
	rec := httptest.NewRecorder()
	e.Response = rec

	if err := downloadGalleryZip(e, event); err != nil {
		t.Fatalf("downloadGalleryZip: %v", err)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", ct)
	}

	body := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("parse zip (%d bytes): %v", len(body), err)
	}

	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %q: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read entry %q: %v", f.Name, err)
		}
		out[f.Name] = data
	}
	return out, len(body)
}

func newUpload(t *testing.T, app core.App, event, prompt *core.Record, guestName string, imageBytes []byte, imageName string, displayBytes []byte, displayName string) *core.Record {
	t.Helper()
	coll, err := app.FindCollectionByNameOrId("uploads")
	if err != nil {
		t.Fatal(err)
	}
	u := core.NewRecord(coll)
	u.Set("event", event.Id)
	u.Set("prompt", prompt.Id)
	if guestName != "" {
		u.Set("guest_name", guestName)
	}
	img, err := filesystem.NewFileFromBytes(imageBytes, imageName)
	if err != nil {
		t.Fatal(err)
	}
	u.Set("image", img)
	if displayBytes != nil {
		disp, err := filesystem.NewFileFromBytes(displayBytes, displayName)
		if err != nil {
			t.Fatal(err)
		}
		u.Set("display", disp)
	}
	if err := app.Save(u); err != nil {
		t.Fatalf("save upload: %v", err)
	}
	return u
}

// TestDownloadGalleryZip is the regression guard for the empty-gallery-ZIP bug:
// the handler must stream every stored upload's bytes back, reading through the
// app's configured filesystem. Previously it read straight off local disk with
// os.Open, which returned nothing whenever S3 storage was enabled and produced
// a silent 22-byte empty archive for every event.
//
// It also locks in the entry-naming rules the gallery depends on: a flat
// numeric prefix, guest-name folding, same-name de-duplication, and exporting
// the uploaded original even when a browser rendition exists.
func TestDownloadGalleryZip(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	owner := core.NewRecord(users)
	owner.Set("email", "host@example.com")
	owner.SetPassword("password1234")
	owner.Set("tier", "premium")
	if err := app.Save(owner); err != nil {
		t.Fatal(err)
	}

	events, err := app.FindCollectionByNameOrId("events")
	if err != nil {
		t.Fatal(err)
	}
	event := core.NewRecord(events)
	event.Set("title", "Anna & Marc")
	event.Set("owner", owner.Id)
	if err := app.Save(event); err != nil {
		t.Fatal(err)
	}

	promptsColl, err := app.FindCollectionByNameOrId("prompts")
	if err != nil {
		t.Fatal(err)
	}
	newPrompt := func(order int, text string) *core.Record {
		p := core.NewRecord(promptsColl)
		p.Set("event", event.Id)
		p.Set("text", text)
		p.Set("sort_order", order)
		if err := app.Save(p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p1 := newPrompt(1, "First dance")
	p2 := newPrompt(2, "The rings")

	// Uploads may still live on inherited prompt rows, but the archive must be
	// flat and never expose those internal labels. The third file also checks a
	// HEIC upload with a JPEG rendition and a non-ASCII guest name.
	newUpload(t, app, event, p1, "", []byte("PHOTO-ONE"), "one.jpg", nil, "")
	newUpload(t, app, event, p1, "", []byte("PHOTO-TWO"), "two.jpg", nil, "")
	newUpload(t, app, event, p2, "Alice Müller", []byte("HEIC-ORIGINAL"), "shot.heic", []byte("JPEG-RENDITION"), "shot.jpg")

	entries, size := readGalleryZip(t, app, event)

	// The headline regression: a populated archive, not the 22-byte empty one.
	if size <= 22 {
		t.Fatalf("archive is %d bytes — looks empty (the original bug)", size)
	}
	if len(entries) != 3 {
		names := make([]string, 0, len(entries))
		for n := range entries {
			names = append(names, n)
		}
		sort.Strings(names)
		t.Fatalf("got %d entries %v, want 3", len(entries), names)
	}

	var contents []string
	foundNamedOriginal := false
	for name, body := range entries {
		if strings.Contains(name, "First dance") || strings.Contains(name, "The rings") {
			t.Errorf("archive name %q leaked an internal bucket label", name)
		}
		contents = append(contents, string(body))
		if strings.Contains(name, "Alice Müller") && strings.HasSuffix(name, ".heic") && string(body) == "HEIC-ORIGINAL" {
			foundNamedOriginal = true
		}
	}
	sort.Strings(contents)
	if want := []string{"HEIC-ORIGINAL", "PHOTO-ONE", "PHOTO-TWO"}; !slices.Equal(contents, want) {
		t.Errorf("archive contents = %v, want %v", contents, want)
	}
	if !foundNamedOriginal {
		t.Fatalf("missing guest-named original; got keys %v", keys(entries))
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

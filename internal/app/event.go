package app

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

type dbxParams = dbx.Params

// galleryBucketText is internal plumbing only. Every gallery has one hidden
// prompt row because uploads in the inherited schema require a prompt ID.
// The value is never rendered or exported to customers.
const galleryBucketText = "Gallery uploads"

func eventStoredLang(event *core.Record) string {
	if l := event.GetString("lang"); i18n.IsSupported(l) {
		return l
	}
	return i18n.DefaultLang
}

func defaultEventLang(e *core.RequestEvent) string {
	if e.Auth != nil {
		if l := e.Auth.GetString("signup_lang"); i18n.IsSupported(l) {
			return l
		}
	}
	if urlLang, _ := i18n.FromPath(e.Request.URL.Path); i18n.IsSupported(urlLang) {
		return urlLang
	}
	return i18n.DefaultLang
}

// createEvent creates the gallery and its single hidden upload bucket. There
// is deliberately no prompt, QR-mode, or theme input in the product model.
func createEvent(e *core.RequestEvent, title, eventDate string) (*core.Record, error) {
	events, err := e.App.FindCollectionByNameOrId("events")
	if err != nil {
		return nil, err
	}
	prompts, err := e.App.FindCollectionByNameOrId("prompts")
	if err != nil {
		return nil, err
	}

	event := core.NewRecord(events)
	event.Set("title", title)
	event.Set("event_date", eventDate)
	event.Set("lang", defaultEventLang(e))
	event.Set("owner", e.Auth.Id)
	// Existing pre-redesign databases may still carry the old required fields.
	// Populate harmless compatibility values without exposing either concept in
	// the current product or adding them to fresh schemas.
	if event.Collection().Fields.GetByName("design_id") != nil {
		event.Set("design_id", "classic")
	}
	if event.Collection().Fields.GetByName("single_qr_mode") != nil {
		event.Set("single_qr_mode", true)
	}

	if err := e.App.RunInTransaction(func(tx core.App) error {
		if err := tx.Save(event); err != nil {
			return err
		}
		bucket := core.NewRecord(prompts)
		bucket.Set("event", event.Id)
		bucket.Set("text", galleryBucketText)
		bucket.Set("sort_order", 1)
		return tx.Save(bucket)
	}); err != nil {
		return nil, err
	}

	return event, nil
}

// findGalleryBucket returns the one internal prompt used as the upload
// destination. For old multi-prompt records it consistently chooses the first
// bucket; all existing uploads still appear in the flat gallery.
func findGalleryBucket(app core.App, eventID string) (*core.Record, error) {
	rows, err := app.FindRecordsByFilter(
		"prompts",
		"event = {:eid}",
		"sort_order,created",
		1,
		0,
		dbxParams{"eid": eventID},
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("gallery has no upload bucket")
	}
	return rows[0], nil
}

func deletePromptUploads(app core.App, promptID string) error {
	uploads, err := app.FindRecordsByFilter("uploads", "prompt = {:pid}", "", 0, 0, dbxParams{"pid": promptID})
	if err != nil {
		return err
	}
	for _, upload := range uploads {
		if err := app.Delete(upload); err != nil {
			return err
		}
	}
	return nil
}

func findOwnedEvents(e *core.RequestEvent) []*core.Record {
	if e.Auth == nil {
		return nil
	}
	records, err := e.App.FindRecordsByFilter(
		"events",
		"owner = {:uid}",
		"-created",
		200,
		0,
		dbxParams{"uid": e.Auth.Id},
	)
	if err != nil {
		return nil
	}
	return records
}

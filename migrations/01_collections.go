package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// The complete schema in a single migration. This template starts every new
// product from a fresh database, so there is no history to preserve — evolve
// this file freely while your product is unlaunched, then switch to additive
// numbered migrations once real data exists.
//
// Collections:
//
//   - users    (auth) — hosts. tier drives feature gating (see helpers.go).
//   - events   — one per gallery. Owns one hidden upload bucket and its files.
//   - prompts  — inherited upload-bucket storage. The product creates exactly
//     one per event and never exposes it in the UI.
//   - uploads  — guest submissions. `image` is the original file; `display`
//     is an optional browser-friendly JPEG rendition (HEIC transcodes).
//
// References between collections are plain text ID fields, not PocketBase
// relation fields: every access path goes through our handlers (which join
// manually), and the public record API is locked to superusers below, so
// relation expansion would buy nothing.
const adminThumbSize = "480x480"

func init() {
	m.Register(func(app core.App) error {
		// PocketBase ships a default "users" auth collection, so this always
		// finds one — we extend it with our custom fields (idempotently, so
		// re-running against an existing database is safe).
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			users = core.NewAuthCollection("users")
		}
		users.PasswordAuth.Enabled = true
		userFields := []core.Field{
			&core.TextField{Name: "name"},
			&core.TextField{Name: "tier", Required: true},
			// The language and country the user signed up in/from — used to
			// default new events and for lifecycle-email / analytics targeting.
			&core.TextField{Name: "signup_lang"},
			&core.TextField{Name: "signup_country"},
			// "email" or "google" — how the account was created.
			&core.TextField{Name: "auth_provider"},
			// Opt-out flag for marketing/lifecycle email (see ADAPTING.md for
			// porting the retargeting campaign machinery that consumes it).
			&core.BoolField{Name: "marketing_opt_out"},
		}
		for _, f := range userFields {
			if users.Fields.GetByName(f.GetName()) == nil {
				users.Fields.Add(f)
			}
		}
		if err := app.Save(users); err != nil {
			return err
		}

		if coll, _ := app.FindCollectionByNameOrId("events"); coll == nil {
			events := core.NewBaseCollection("events")
			events.Fields.Add(&core.TextField{Name: "title", Required: true})
			events.Fields.Add(&core.TextField{Name: "event_date"})
			events.Fields.Add(&core.TextField{Name: "lang"})
			// Commercial guest controls only take effect while the owner remains
			// on that plan (see helpers.go).
			events.Fields.Add(&core.BoolField{Name: "disable_guest_download"})
			events.Fields.Add(&core.BoolField{Name: "collect_guest_name"})
			events.Fields.Add(&core.TextField{Name: "owner", Required: true})
			events.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
			events.Fields.Add(&core.AutodateField{Name: "updated", OnUpdate: true})
			events.AddIndex("idx_events_owner", false, "owner", "")
			if err := app.Save(events); err != nil {
				return err
			}
		}

		if coll, _ := app.FindCollectionByNameOrId("prompts"); coll == nil {
			prompts := core.NewBaseCollection("prompts")
			prompts.Fields.Add(&core.TextField{Name: "event", Required: true})
			prompts.Fields.Add(&core.TextField{Name: "text", Required: true})
			// Kept for compatibility with the inherited schema; new galleries
			// always store one bucket at position 1.
			prompts.Fields.Add(&core.NumberField{Name: "sort_order"})
			prompts.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
			prompts.Fields.Add(&core.AutodateField{Name: "updated", OnUpdate: true})
			prompts.AddIndex("idx_prompts_event", false, "event", "")
			if err := app.Save(prompts); err != nil {
				return err
			}
		}

		if coll, _ := app.FindCollectionByNameOrId("uploads"); coll == nil {
			uploads := core.NewBaseCollection("uploads")
			// Thumbs let the admin dashboard request small centre-cropped
			// renditions via ?thumb= instead of full-res originals. PocketBase
			// only generates sizes listed here; keep in sync with admin.html.
			// Originals are never recompressed. A generous per-file ceiling lets
			// guests send phone video while the handler enforces the 100 GB
			// aggregate gallery allowance.
			uploads.Fields.Add(&core.FileField{Name: "image", Required: true, MaxSelect: 1, MaxSize: 2147483648, Thumbs: []string{adminThumbSize}})
			uploads.Fields.Add(&core.FileField{Name: "display", MaxSelect: 1, MaxSize: 52428800, Thumbs: []string{adminThumbSize}})
			uploads.Fields.Add(&core.TextField{Name: "prompt", Required: true})
			uploads.Fields.Add(&core.TextField{Name: "event", Required: true})
			uploads.Fields.Add(&core.TextField{Name: "guest_name", Max: 60})
			uploads.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
			uploads.Fields.Add(&core.AutodateField{Name: "updated", OnUpdate: true})
			uploads.AddIndex("idx_uploads_prompt", false, "prompt", "")
			uploads.AddIndex("idx_uploads_event", false, "event", "")
			if err := app.Save(uploads); err != nil {
				return err
			}
		}

		// Lock every collection's record API rules to superuser-only (nil).
		//
		// All reads and writes in this app go through server-side handlers,
		// which bypass API rules entirely — the rules only govern the public
		// /api/collections/... record endpoints, which nothing client-side
		// uses. Leaving them open would allow anyone to enumerate users
		// (emails), events and uploads across all accounts, or to create a
		// user with an arbitrary tier.
		//
		// Guest galleries are unaffected: they load images via /api/files, and
		// the uploads file fields are not Protected, so files stay accessible
		// by known URL regardless of the collection rules.
		for _, name := range []string{"users", "events", "prompts", "uploads"} {
			coll, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			coll.ListRule = nil
			coll.ViewRule = nil
			coll.CreateRule = nil
			coll.UpdateRule = nil
			coll.DeleteRule = nil
			if coll.IsAuth() {
				coll.ManageRule = nil
			}
			if err := app.Save(coll); err != nil {
				return err
			}
		}

		// Enable application logging so the admin dashboard's Logs tab has
		// data. PocketBase treats logs.maxDays == 0 as "logging disabled".
		s := app.Settings()
		if s.Logs.MaxDays == 0 {
			s.Logs.MaxDays = 14
		}
		if s.Logs.MinLevel > 0 {
			s.Logs.MinLevel = 0
		}
		return app.Save(s)
	}, func(app core.App) error {
		for _, name := range []string{"uploads", "prompts", "events", "users"} {
			coll, _ := app.FindCollectionByNameOrId(name)
			if coll != nil {
				app.Delete(coll)
			}
		}
		return nil
	})
}

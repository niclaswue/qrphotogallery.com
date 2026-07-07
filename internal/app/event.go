package app

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

type dbxParams = dbx.Params

// eventStoredLang returns the supported language the event was created in
// (from the lang field set at creation), falling back to the default
// language for records that hold an unknown value. This is the language
// guests see by default and the one the printed cards/poster are rendered in.
func eventStoredLang(event *core.Record) string {
	if l := event.GetString("lang"); i18n.IsSupported(l) {
		return l
	}
	return i18n.DefaultLang
}

// defaultEventLang picks the language a freshly created event should default
// to: the language the owner signed up in (stored on the user record), which
// is what guests see and what the printed cards/QR codes are rendered in.
// We fall back to the language the create flow is being browsed in, then to
// the default language, so the field is always a supported value.
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

// pickPrompt selects the next prompt to hand a guest in single-QR mode.
//
//   - seen holds the prompt IDs this guest's browser has already been shown
//     (from the per-event guest cookie).
//   - hasUpload holds the prompt IDs that already have at least one upload
//     (event-wide). It only matters once the guest has seen everything, so
//     callers may pass an empty/lazily-built map while unseen prompts remain.
//
// The two-phase logic: first cover prompts this guest hasn't seen, then —
// once they've seen them all — steer toward prompts that still need an
// upload, breaking ties by the fewest global shows (show_count).
//
// Within phase one we still prefer unseen prompts that have no upload yet.
// That maximises coverage and, on the free tier (one upload per prompt),
// avoids handing a guest a prompt whose single slot is already filled —
// which the upload handler would otherwise reject. Returns nil only if
// prompts is empty.
func pickPrompt(prompts []*core.Record, seen map[string]bool, hasUpload map[string]bool) *core.Record {
	if len(prompts) == 0 {
		return nil
	}

	var unseen []*core.Record
	for _, p := range prompts {
		if !seen[p.Id] {
			unseen = append(unseen, p)
		}
	}

	if len(unseen) > 0 {
		// Prefer unseen prompts that still need an upload; fall back to any unseen.
		var unseenEmpty []*core.Record
		for _, p := range unseen {
			if !hasUpload[p.Id] {
				unseenEmpty = append(unseenEmpty, p)
			}
		}
		if len(unseenEmpty) > 0 {
			return unseenEmpty[rand.IntN(len(unseenEmpty))]
		}
		return unseen[rand.IntN(len(unseen))]
	}

	// Phase two: guest has seen everything. Focus on prompts without an
	// upload; if every prompt already has one, fall back to the whole set so
	// the event keeps cycling rather than dead-ending.
	pool := make([]*core.Record, 0, len(prompts))
	for _, p := range prompts {
		if !hasUpload[p.Id] {
			pool = append(pool, p)
		}
	}
	if len(pool) == 0 {
		pool = prompts
	}

	// Keep only the prompts tied for the fewest global shows.
	minShows := pool[0].GetInt("show_count")
	for _, p := range pool {
		if s := p.GetInt("show_count"); s < minShows {
			minShows = s
		}
	}
	var leastShown []*core.Record
	for _, p := range pool {
		if p.GetInt("show_count") == minShows {
			leastShown = append(leastShown, p)
		}
	}
	return leastShown[rand.IntN(len(leastShown))]
}

// createEvent creates an event plus its prompt rows in one go. The prompts
// are inserted in a transaction; on failure the event record is rolled back
// too so a half-created event never lingers.
func createEvent(e *core.RequestEvent, title, eventDate string, prompts []string, designID string, singleQR bool) (*core.Record, error) {
	collection, err := e.App.FindCollectionByNameOrId("events")
	if err != nil {
		return nil, err
	}

	event := core.NewRecord(collection)
	event.Set("title", title)
	event.Set("event_date", eventDate)
	event.Set("design_id", designID)
	event.Set("single_qr_mode", singleQR)
	event.Set("lang", defaultEventLang(e))
	event.Set("owner", e.Auth.Id)

	if err := e.App.Save(event); err != nil {
		return nil, err
	}

	promptColl, err := e.App.FindCollectionByNameOrId("prompts")
	if err != nil {
		e.App.Delete(event)
		return nil, err
	}

	txErr := e.App.RunInTransaction(func(txApp core.App) error {
		for i, text := range prompts {
			p := core.NewRecord(promptColl)
			p.Set("event", event.Id)
			p.Set("text", text)
			p.Set("sort_order", i+1)
			if err := txApp.Save(p); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		e.App.Delete(event)
		return nil, txErr
	}

	return event, nil
}

// promptInput is one prompt as submitted from the edit form. ID is the
// existing prompt record's ID (empty for a newly added prompt), which lets us
// edit prompts by identity rather than by list position.
type promptInput struct {
	ID   string
	Text string
}

// updateEventPrompts reconciles an event's prompts against the edited list,
// matching by prompt ID so each prompt — and the uploads bound to it —
// stays correctly attached across edits:
//
//   - prompts present in the submission keep their existing row (so the prompt
//     ID stays stable and printed QR codes keep working) and have their text
//     and order updated;
//   - newly added prompts (empty ID) get a fresh row;
//   - prompts the owner removed are deleted *together with their uploads*, so
//     a deleted prompt never leaves an orphaned upload behind, and an edit
//     never re-binds an existing upload onto a different prompt's text.
func updateEventPrompts(e *core.RequestEvent, eventID string, prompts []promptInput) error {
	event, err := e.App.FindRecordById("events", eventID)
	if err != nil {
		return err
	}

	if e.Auth == nil || e.Auth.Id != event.GetString("owner") {
		return fmt.Errorf("not authorized")
	}

	tier := getUserTier(e.Auth)
	if len(prompts) > tier.MaxPrompts {
		return fmt.Errorf("exceeds tier limit")
	}

	existing, err := e.App.FindRecordsByFilter("prompts", "event = {:eid}", "sort_order", 1000, 0, dbxParams{"eid": eventID})
	if err != nil {
		return err
	}
	existingByID := make(map[string]*core.Record, len(existing))
	for _, p := range existing {
		existingByID[p.Id] = p
	}

	return e.App.RunInTransaction(func(txApp core.App) error {
		promptColl, err := txApp.FindCollectionByNameOrId("prompts")
		if err != nil {
			return err
		}

		kept := make(map[string]bool, len(prompts))
		for i, in := range prompts {
			if p, ok := existingByID[in.ID]; ok {
				p.Set("text", in.Text)
				p.Set("sort_order", i+1)
				if err := txApp.Save(p); err != nil {
					return err
				}
				kept[p.Id] = true
				continue
			}
			// No matching existing row (new prompt, or an ID that no longer
			// exists) — create a fresh one.
			p := core.NewRecord(promptColl)
			p.Set("event", eventID)
			p.Set("text", in.Text)
			p.Set("sort_order", i+1)
			if err := txApp.Save(p); err != nil {
				return err
			}
		}

		// Anything the owner dropped from the list is deleted along with its
		// uploads, so removing a prompt also removes its submissions.
		for _, p := range existing {
			if kept[p.Id] {
				continue
			}
			if err := deletePromptUploads(txApp, p.Id); err != nil {
				return err
			}
			if err := txApp.Delete(p); err != nil {
				return err
			}
		}
		return nil
	})
}

// deletePromptUploads removes every upload (and its stored files) belonging to
// a prompt. Used when a prompt is deleted so no submission is left orphaned.
func deletePromptUploads(app core.App, promptID string) error {
	uploads, err := app.FindRecordsByFilter("uploads", "prompt = {:pid}", "", 0, 0, dbxParams{"pid": promptID})
	if err != nil {
		return err
	}
	for _, u := range uploads {
		if err := app.Delete(u); err != nil {
			return err
		}
	}
	return nil
}

func updateEventDesign(e *core.RequestEvent, eventID string, designID string) error {
	event, err := e.App.FindRecordById("events", eventID)
	if err != nil {
		return err
	}
	if e.Auth == nil || e.Auth.Id != event.GetString("owner") {
		return fmt.Errorf("not authorized")
	}
	if GetDesignByID(designID) == nil {
		return fmt.Errorf("invalid design selected")
	}
	event.Set("design_id", designID)
	return e.App.Save(event)
}

// findOwnedEvents returns every event owned by the authenticated user,
// newest first. Returns nil for unauthenticated requests.
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

func parsePrompts(raw string) []string {
	var result []string
	for _, p := range strings.Split(raw, "\n") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parsePromptInputs decodes the edit form's JSON prompt payload — a list of
// {id, text} objects — into promptInputs, trimming text, dropping blanks and
// clamping over-long entries. Returns nil when the payload is missing or
// malformed so callers can fall back to the plain newline field.
func parsePromptInputs(raw string) []promptInput {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var decoded []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil
	}
	result := make([]promptInput, 0, len(decoded))
	for _, d := range decoded {
		text := strings.TrimSpace(d.Text)
		if text == "" {
			continue
		}
		if len(text) > 120 {
			text = text[:120]
		}
		result = append(result, promptInput{ID: strings.TrimSpace(d.ID), Text: text})
	}
	return result
}

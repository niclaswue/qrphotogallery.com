package app

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

var cardLabelKeys = [...]string{
	"photo",
	"how_it_works",
	"instruction_lead",
	"instruction_emph",
	"instruction_tail",
	"no",
}

func cardLabels(lang string) map[string]string {
	labels := make(map[string]string, len(cardLabelKeys))
	for _, k := range cardLabelKeys {
		labels[k] = i18n.T(lang, "card."+k)
	}
	return labels
}

// buildPosterRender resolves the poster's content: the event's design palette
// plus localized text. The footer is always the app URL — it is not
// user-editable on purpose, so every printed poster carries the attribution.
func buildPosterRender(design *Design, title, lang, appURL string) *posterRender {
	return &posterRender{
		Title:   title,
		Heading: i18n.T(lang, "poster.title"),
		Caption: i18n.T(lang, "poster.instruction"),
		ScanMe:  i18n.T(lang, "poster.scan_me"),
		Footer:  stripURLDisplay(appURL),
	}
}

// RenderCardPNG renders one side ("front" or "back") of a single prompt card
// at the given pixel density — the preview-cards CLI goes through here, using
// the same Typst template (and its preview-front / preview-back single-card
// modes) as the printed deck, so the preview matches the PDF exactly. qrURL
// is what the back-side QR encodes.
func RenderCardPNG(design Design, side, title, prompt, lang, qrURL, appURL string, ppi int) ([]byte, error) {
	if appURL == "" && appConfig != nil {
		appURL = appConfig.AppURL
	}
	mode := "preview-front"
	if side == "back" {
		mode = "preview-back"
	}
	job := printJob{
		EventTitle:    title,
		AppURL:        appURL,
		AppURLDisplay: stripURLDisplay(appURL),
		Labels:        cardLabels(lang),
		Design: printDesign{
			ID:         design.ID,
			Name:       design.Name,
			Primary:    design.Primary,
			Secondary:  design.Secondary,
			Accent:     design.Accent,
			Background: design.Background,
			Text:       design.Text,
		},
		Prompts: []printPrompt{{ID: "preview", Text: prompt, SortOrder: 1, ShortURL: qrURL}},
	}
	return renderTypstCards(design.ID, job,
		"--input", "mode="+mode,
		"--format", "png",
		"--ppi", fmt.Sprintf("%d", ppi))
}

func generateCardsPDF(e *core.RequestEvent, event *core.Record, prompts []*core.Record, design *Design, lang string) error {
	// Cards are rendered in the event's language, and each QR pins that
	// language via ?lang= so a scan lands on the upload page in the same
	// language regardless of the guest's browser.
	job := printJob{
		EventTitle:    event.GetString("title"),
		EventDate:     event.GetString("event_date"),
		AppURL:        appConfig.AppURL,
		AppURLDisplay: stripURLDisplay(appConfig.AppURL),
		Labels:        cardLabels(lang),
		Design: printDesign{
			ID:         design.ID,
			Name:       design.Name,
			Primary:    design.Primary,
			Secondary:  design.Secondary,
			Accent:     design.Accent,
			Background: design.Background,
			Text:       design.Text,
		},
		Prompts: make([]printPrompt, 0, len(prompts)),
	}

	for _, p := range prompts {
		job.Prompts = append(job.Prompts, printPrompt{
			ID:        p.Id,
			Text:      p.GetString("text"),
			SortOrder: p.GetInt("sort_order"),
			ShortURL:  fmt.Sprintf("%s/e/%s/%s?lang=%s", appConfig.AppURL, event.Id, p.Id, lang),
		})
	}

	pdfBytes, err := renderTypstCards(design.ID, job)
	if err != nil {
		return e.InternalServerError("Failed to render PDF", err)
	}

	e.Response.Header().Set("Content-Type", "application/pdf")
	e.Response.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s-cards-%s.pdf\"", sanitizeFilename(event.GetString("title")), lang))
	_, err = e.Response.Write(pdfBytes)
	return err
}

// generatePosterPDF renders the single-QR poster for an event: one page with
// a large QR pointing at the dispatcher (app_url/e/{id}) for the single-QR
// distribution mode, themed by the event's design palette. The app_url must
// be the public URL, since it's what guests scan — same constraint as the
// per-prompt cards.
func generatePosterPDF(e *core.RequestEvent, event *core.Record, design *Design, lang string) error {
	job := printJob{
		EventTitle:    event.GetString("title"),
		EventDate:     event.GetString("event_date"),
		AppURL:        appConfig.AppURL,
		AppURLDisplay: stripURLDisplay(appConfig.AppURL),
		EventURL:      fmt.Sprintf("%s/e/%s?lang=%s", appConfig.AppURL, event.Id, lang),
		Design: printDesign{
			ID:         design.ID,
			Name:       design.Name,
			Primary:    design.Primary,
			Secondary:  design.Secondary,
			Accent:     design.Accent,
			Background: design.Background,
			Text:       design.Text,
		},
		Poster: buildPosterRender(design, event.GetString("title"), lang, appConfig.AppURL),
	}

	pdfBytes, err := renderTypstPoster(job)
	if err != nil {
		return e.InternalServerError("Failed to render PDF", err)
	}

	e.Response.Header().Set("Content-Type", "application/pdf")
	e.Response.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s-poster-%s.pdf\"", sanitizeFilename(event.GetString("title")), lang))
	_, err = e.Response.Write(pdfBytes)
	return err
}

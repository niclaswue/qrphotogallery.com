package app

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// posterPalette is fixed product styling, not a host-selectable theme.
var posterPalette = printPalette{
	Primary:    "#E6634F",
	Secondary:  "#DCE6E8",
	Accent:     "#E6634F",
	Background: "#F7F5EF",
	Text:       "#11243A",
}

func buildPosterRender(title, lang, appURL string) *posterRender {
	return &posterRender{
		Title:   title,
		Heading: i18n.T(lang, "poster.title"),
		Caption: i18n.T(lang, "poster.instruction"),
		ScanMe:  i18n.T(lang, "poster.scan_me"),
		Footer:  stripURLDisplay(appURL),
	}
}

func generatePosterPDF(e *core.RequestEvent, event *core.Record, lang string) error {
	job := printJob{
		EventTitle:    event.GetString("title"),
		EventDate:     event.GetString("event_date"),
		AppURL:        appConfig.AppURL,
		AppURLDisplay: stripURLDisplay(appConfig.AppURL),
		EventURL:      fmt.Sprintf("%s/e/%s?lang=%s", appConfig.AppURL, event.Id, lang),
		Palette:       posterPalette,
		Poster:        buildPosterRender(event.GetString("title"), lang, appConfig.AppURL),
	}
	pdfBytes, err := renderTypstPoster(job)
	if err != nil {
		return e.InternalServerError("Failed to render PDF", err)
	}
	e.Response.Header().Set("Content-Type", "application/pdf")
	e.Response.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s-qr-poster.pdf\"", sanitizeFilename(event.GetString("title"))))
	_, err = e.Response.Write(pdfBytes)
	return err
}

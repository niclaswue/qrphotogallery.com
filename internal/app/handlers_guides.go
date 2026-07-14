package app

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// hubGuide is one card on the /guides index page.
type hubGuide struct {
	Slug     string
	Category string
	Title    string
	Intro    string
	Updated  string
}

// handleGuideIndex renders the guides hub — the topical-authority landing
// page that links to every guide and funnels toward /create.
func handleGuideIndex(e *core.RequestEvent) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)

	var wedding, occasion []hubGuide
	for _, slug := range guideSlugs {
		gc, ok := lookupGuide(slug, lang)
		if !ok {
			continue
		}
		card := hubGuide{
			Slug:     gc.Slug,
			Category: gc.Category,
			Title:    gc.H1,
			Intro:    gc.Description,
			Updated:  gc.Updated,
		}
		if gc.Category == "wedding" {
			wedding = append(wedding, card)
		} else {
			occasion = append(occasion, card)
		}
	}

	stdCents, _ := tierPriceCents(appConfig.Tiers)

	baseURL := trimmedBaseURL()
	canonical := baseURL + i18n.LangPath(lang) + "/guides"
	indexJSONLD := guideIndexJSONLD(lang, baseURL, canonical,
		i18n.T(lang, "guides.index.h1"), i18n.T(lang, "guides.index.meta_description"),
		i18n.T(lang, "guides.breadcrumb_home"), i18n.T(lang, "guides.breadcrumb"))

	return e.HTML(http.StatusOK, renderWithBase(e, "guides_index", map[string]any{
		"Title":         i18n.T(lang, "guides.index.title"),
		"Description":   i18n.T(lang, "guides.index.meta_description"),
		"WeddingGuides": wedding,
		"Occasion":      occasion,
		"StandardPrice": formatTierPrice(stdCents, lang),
		"IndexJSONLD":   indexJSONLD,
	}))
}

// handleGuide renders a single guide. Unknown slugs get an explicit 404 —
// without it the ServeMux "/" catch-all would fall through to the landing
// page (see AGENTS.md "Things that bite").
func handleGuide(e *core.RequestEvent) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	slug := e.Request.PathValue("slug")

	gc, ok := lookupGuide(slug, lang)
	if !ok {
		return renderHTMLErrorKeys(e, http.StatusNotFound, "error.title.not_found", "error.message.page_not_found")
	}

	baseURL := trimmedBaseURL()
	canonical := baseURL + i18n.LangPath(lang) + "/guides/" + slug
	jsonLD := guideJSONLD(gc, lang, baseURL, canonical,
		i18n.T(lang, "guides.breadcrumb_home"), i18n.T(lang, "guides.breadcrumb"))

	stdCents, _ := tierPriceCents(appConfig.Tiers)

	return e.HTML(http.StatusOK, renderWithBase(e, "guide", map[string]any{
		"Title":         gc.Title,
		"Description":   gc.Description,
		"GuideH1":       gc.H1,
		"GuideIntro":    gc.Intro,
		"GuideUpdated":  gc.Updated,
		"GuideBody":     gc.Body,
		"GuideFAQ":      gc.FAQ,
		"GuideCategory": gc.Category,
		"GuideJSONLD":   jsonLD,
		"Related":       relatedGuidesFor(slug, lang, 3),
		"StandardPrice": formatTierPrice(stdCents, lang),
	}))
}

// trimmedBaseURL returns the configured app URL without a trailing slash.
func trimmedBaseURL() string {
	url := appConfig.AppURL
	for len(url) > 0 && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	return url
}

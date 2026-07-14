package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// SEO content hub. Guides are long-form marketing/answer pages that build
// topical authority around the product's core queries (e.g. "qr photo album
// wedding", "qr photo gallery wedding") and the occasions people buy for
// (weddings, prom/Abiball, birthdays, corporate events, graduations).
//
// One guide = one Markdown file per language under data/guides/. This mirrors
// the legal-content pattern (see legal.go): structured, per-language source,
// rendered once at startup. Each file carries a small key: value frontmatter
// block for the title/description/FAQ (which power <title>, meta tags and
// JSON-LD) followed by the Markdown body.
//
// Internal links inside a guide are written with the %LP% token, which is
// replaced at load time with the language URL prefix ("" for the default
// language, "/de" for German) so a German guide links to German pages.

// guideDef declares one guide, its render/sitemap order and its sitemap
// priority. Add a guide by dropping data/guides/<slug>.md (+ .de.md) on disk
// and appending an entry here — routes, sitemap, hub page and footer pick it
// up automatically.
type guideDef struct {
	Slug     string
	Category string // grouping label key suffix: "wedding" or "occasion"
	Priority string
}

// guideRegistry is the single source of truth for which guides exist and in
// what order. Wedding pages lead (highest commercial intent), occasion pages
// follow to widen topical coverage.
var guideRegistry = []guideDef{
	{Slug: "wedding-qr-photo-album", Category: "wedding", Priority: "0.9"},
	{Slug: "wedding-qr-photo-gallery", Category: "wedding", Priority: "0.9"},
	{Slug: "collect-photos-from-wedding-guests", Category: "wedding", Priority: "0.8"},
	{Slug: "prom-qr-photo-sharing", Category: "occasion", Priority: "0.7"},
	{Slug: "birthday-qr-photo-gallery", Category: "occasion", Priority: "0.7"},
	{Slug: "corporate-event-photo-sharing", Category: "occasion", Priority: "0.7"},
	{Slug: "graduation-qr-photo-gallery", Category: "occasion", Priority: "0.7"},
}

// faqItem is one question/answer pair, rendered both as a visible <details>
// block and as a schema.org Question inside the page's FAQPage JSON-LD.
type faqItem struct {
	Q string
	A string
}

// guideContent is one guide rendered for one language.
type guideContent struct {
	Slug        string
	Category    string
	Priority    string
	Title       string
	Description string
	H1          string
	Intro       string
	Keywords    string
	Updated     string
	Body        template.HTML
	FAQ         []faqItem
}

// guides holds every guide keyed by slug then language, populated once at
// startup by loadGuides. guideSlugs preserves registry order for the hub
// page, sitemap and footer.
var (
	guides     = map[string]map[string]*guideContent{}
	guideSlugs []string
)

// loadGuides reads and renders every registered guide for every supported
// language. Called once at startup; editing a guide needs a restart, same as
// translations and legal copy.
func loadGuides() error {
	md := goldmark.New()
	guides = map[string]map[string]*guideContent{}
	guideSlugs = guideSlugs[:0]

	for _, def := range guideRegistry {
		guideSlugs = append(guideSlugs, def.Slug)
		guides[def.Slug] = map[string]*guideContent{}
		for _, lang := range i18n.SupportedLangs {
			filename := def.Slug + ".md"
			if lang != i18n.DefaultLang {
				filename = def.Slug + "." + lang + ".md"
			}
			path := filepath.Join("data", "guides", filename)
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read guide %s: %w", path, err)
			}
			// Rewrite internal-link tokens to this language's URL prefix
			// before anything else so both frontmatter and body are covered.
			raw = bytes.ReplaceAll(raw, []byte("%LP%"), []byte(i18n.LangPath(lang)))

			meta, body := splitFrontmatter(raw)
			var buf bytes.Buffer
			if err := md.Convert(body, &buf); err != nil {
				return fmt.Errorf("render guide %s: %w", path, err)
			}

			gc := &guideContent{
				Slug:        def.Slug,
				Category:    def.Category,
				Priority:    def.Priority,
				Title:       meta["title"],
				Description: meta["description"],
				H1:          meta["h1"],
				Intro:       meta["intro"],
				Keywords:    meta["keywords"],
				Updated:     meta["updated"],
				Body:        template.HTML(buf.String()),
				FAQ:         parseFAQ(meta),
			}
			if gc.Title == "" || gc.Description == "" || gc.H1 == "" {
				return fmt.Errorf("guide %s is missing title, description or h1 frontmatter", path)
			}
			guides[def.Slug][lang] = gc
		}
	}
	return nil
}

// lookupGuide returns the guide for slug in lang, falling back to the default
// language, and reports whether the slug exists at all.
func lookupGuide(slug, lang string) (*guideContent, bool) {
	byLang, ok := guides[slug]
	if !ok {
		return nil, false
	}
	if gc, ok := byLang[lang]; ok {
		return gc, true
	}
	if gc, ok := byLang[i18n.DefaultLang]; ok {
		return gc, true
	}
	return nil, false
}

// splitFrontmatter separates a leading "---" fenced key: value block from the
// Markdown body. Values are single-line (no nesting) which keeps the parser
// dependency-free; multi-line prose lives in the body. Returns an empty map
// and the original bytes when no frontmatter is present.
func splitFrontmatter(raw []byte) (map[string]string, []byte) {
	meta := map[string]string{}
	text := string(raw)
	if !strings.HasPrefix(text, "---") {
		return meta, raw
	}
	lines := strings.Split(text, "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return meta, raw
	}
	for i := 1; i < end; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		meta[key] = val
	}
	return meta, []byte(strings.Join(lines[end+1:], "\n"))
}

// parseFAQ collects faq1q/faq1a, faq2q/faq2a, … pairs from the frontmatter in
// order until the first gap. Kept flat (indexed keys rather than a nested
// list) so no YAML dependency is needed.
func parseFAQ(meta map[string]string) []faqItem {
	var out []faqItem
	for i := 1; ; i++ {
		q := meta[fmt.Sprintf("faq%dq", i)]
		a := meta[fmt.Sprintf("faq%da", i)]
		if q == "" || a == "" {
			break
		}
		out = append(out, faqItem{Q: q, A: a})
	}
	return out
}

// relatedGuide is a compact link used in the "related guides" rail.
type relatedGuide struct {
	Slug  string
	Title string
	Intro string
}

// relatedGuidesFor returns up to n guides other than slug, preferring ones in
// the same category, then filling from the rest in registry order.
func relatedGuidesFor(slug, lang string, n int) []relatedGuide {
	self, ok := lookupGuide(slug, lang)
	if !ok {
		return nil
	}
	var same, other []relatedGuide
	for _, s := range guideSlugs {
		if s == slug {
			continue
		}
		gc, ok := lookupGuide(s, lang)
		if !ok {
			continue
		}
		rg := relatedGuide{Slug: gc.Slug, Title: gc.H1, Intro: gc.Description}
		if gc.Category == self.Category {
			same = append(same, rg)
		} else {
			other = append(other, rg)
		}
	}
	combined := append(same, other...)
	if len(combined) > n {
		combined = combined[:n]
	}
	return combined
}

// guideJSONLD builds the combined schema.org @graph (Article + FAQPage +
// BreadcrumbList) for a guide and returns it as a ready-to-embed
// <script type="application/ld+json"> block. json.Marshal escapes <, > and &
// to \u00xx by default, so the output is safe to inline verbatim.
func guideJSONLD(gc *guideContent, lang, baseURL, canonicalURL, homeName, guidesName string) template.HTML {
	langPath := i18n.LangPath(lang)
	logo := baseURL + "/static/img/og-default.jpg"

	article := map[string]any{
		"@type":            "Article",
		"headline":         gc.Title,
		"description":      gc.Description,
		"inLanguage":       lang,
		"mainEntityOfPage": map[string]any{"@type": "WebPage", "@id": canonicalURL},
		"image":            logo,
		"author":           map[string]any{"@type": "Organization", "name": appConfig.AppName, "url": baseURL},
		"publisher": map[string]any{
			"@type": "Organization",
			"name":  appConfig.AppName,
			"url":   baseURL,
			"logo":  map[string]any{"@type": "ImageObject", "url": logo},
		},
	}
	if gc.Updated != "" {
		article["datePublished"] = gc.Updated
		article["dateModified"] = gc.Updated
	}

	graph := []any{article}

	if len(gc.FAQ) > 0 {
		var qs []any
		for _, f := range gc.FAQ {
			qs = append(qs, map[string]any{
				"@type":          "Question",
				"name":           f.Q,
				"acceptedAnswer": map[string]any{"@type": "Answer", "text": f.A},
			})
		}
		graph = append(graph, map[string]any{"@type": "FAQPage", "mainEntity": qs})
	}

	graph = append(graph, map[string]any{
		"@type": "BreadcrumbList",
		"itemListElement": []any{
			map[string]any{"@type": "ListItem", "position": 1, "name": homeName, "item": baseURL + langPath + "/"},
			map[string]any{"@type": "ListItem", "position": 2, "name": guidesName, "item": baseURL + langPath + "/guides"},
			map[string]any{"@type": "ListItem", "position": 3, "name": gc.H1, "item": canonicalURL},
		},
	})

	payload := map[string]any{"@context": "https://schema.org", "@graph": graph}
	buf, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return template.HTML(`<script type="application/ld+json">` + string(buf) + `</script>`)
}

// guideIndexJSONLD builds the schema.org @graph for the /guides hub: a
// CollectionPage carrying an ItemList of every guide, plus a BreadcrumbList.
// This helps answer engines understand the hub → guide relationship and the
// topical cluster as a whole.
func guideIndexJSONLD(lang, baseURL, canonicalURL, name, description, homeName, guidesName string) template.HTML {
	langPath := i18n.LangPath(lang)

	var items []any
	pos := 1
	for _, slug := range guideSlugs {
		gc, ok := lookupGuide(slug, lang)
		if !ok {
			continue
		}
		items = append(items, map[string]any{
			"@type":    "ListItem",
			"position": pos,
			"url":      baseURL + langPath + "/guides/" + slug,
			"name":     gc.H1,
		})
		pos++
	}

	graph := []any{
		map[string]any{
			"@type":       "CollectionPage",
			"name":        name,
			"description": description,
			"inLanguage":  lang,
			"url":         canonicalURL,
			"mainEntity":  map[string]any{"@type": "ItemList", "itemListElement": items},
		},
		map[string]any{
			"@type": "BreadcrumbList",
			"itemListElement": []any{
				map[string]any{"@type": "ListItem", "position": 1, "name": homeName, "item": baseURL + langPath + "/"},
				map[string]any{"@type": "ListItem", "position": 2, "name": guidesName, "item": canonicalURL},
			},
		},
	}

	payload := map[string]any{"@context": "https://schema.org", "@graph": graph}
	buf, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return template.HTML(`<script type="application/ld+json">` + string(buf) + `</script>`)
}

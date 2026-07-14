package app

import (
	"strings"
	"testing"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// TestGuidesLoadForAllLanguages renders every registered guide in every
// supported language and asserts the content is complete. This prevents
// shipping a guide with missing frontmatter, an empty body, no FAQ (which
// would drop the FAQPage schema), or an unsubstituted internal-link token.
func TestGuidesLoadForAllLanguages(t *testing.T) {
	withRepoRoot(t)

	if err := loadGuides(); err != nil {
		t.Fatalf("loadGuides: %v", err)
	}

	for _, def := range guideRegistry {
		for _, lang := range i18n.SupportedLangs {
			byLang, ok := guides[def.Slug]
			if !ok {
				t.Errorf("guide %q not registered after load", def.Slug)
				continue
			}
			gc, ok := byLang[lang]
			if !ok {
				t.Errorf("guide %q is missing the %q language file", def.Slug, lang)
				continue
			}
			if gc.Title == "" || gc.Description == "" || gc.H1 == "" {
				t.Errorf("guide %q (%s) is missing title, description or h1", def.Slug, lang)
			}
			if strings.TrimSpace(string(gc.Body)) == "" {
				t.Errorf("guide %q (%s) rendered an empty body", def.Slug, lang)
			}
			if len(gc.FAQ) == 0 {
				t.Errorf("guide %q (%s) has no FAQ pairs (FAQPage schema would be dropped)", def.Slug, lang)
			}
			if strings.Contains(string(gc.Body), "%LP%") {
				t.Errorf("guide %q (%s) has an unsubstituted %%LP%% link token", def.Slug, lang)
			}
		}
	}
}

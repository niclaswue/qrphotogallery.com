package app

import "testing"

// TestPageTemplatesParse catches syntax errors in pages that are otherwise
// loaded lazily only when their route is first visited.
func TestPageTemplatesParse(t *testing.T) {
	t.Chdir("../..")
	pages := []string{
		"create", "error", "forgot_password", "landing", "legal", "login", "overview",
		"overview_list", "payment_success", "pricing", "register", "upload",
	}
	for _, page := range pages {
		t.Run(page, func(t *testing.T) {
			if _, err := getPageTemplate(page, "en"); err != nil {
				t.Fatalf("parse %s: %v", page, err)
			}
		})
	}
	if _, err := getStandaloneTemplate("demo", "en"); err != nil {
		t.Fatalf("parse standalone demo: %v", err)
	}
}

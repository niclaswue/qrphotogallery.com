package app

import "testing"

// TestPageTemplatesParse catches syntax errors in pages that are otherwise
// loaded lazily only when their route is first visited.
func TestPageTemplatesParse(t *testing.T) {
	t.Chdir("../..")
	pages := []string{
		"challenge", "create", "done", "error", "forgot_password",
		"gallery", "landing", "legal", "library", "login", "overview",
		"overview_list", "payment_success", "pricing", "register", "upload",
	}
	for _, page := range pages {
		t.Run(page, func(t *testing.T) {
			if _, err := getPageTemplate(page); err != nil {
				t.Fatalf("parse %s: %v", page, err)
			}
		})
	}
}

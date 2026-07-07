package app

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/yuin/goldmark"
)

// legalSections holds the rendered HTML for each legal section, populated
// once at startup by loadLegalContent. The page handler reads from this map
// directly — no per-request markdown parsing.
var legalSections = map[string]template.HTML{}

// legalFiles maps the section key used in views/legal.html to a markdown
// filename in data/legal/. Keep the order: this is the render order on the
// page.
var legalFiles = []struct {
	Key, File string
}{
	{"imprint", "imprint.md"},
	{"privacy", "privacy.md"},
	{"refund", "refund.md"},
}

// loadLegalContent reads each markdown file from data/legal/ and renders it
// to HTML. Called once at startup; later changes need a restart, same as
// translations.
func loadLegalContent() error {
	md := goldmark.New()
	for _, entry := range legalFiles {
		path := filepath.Join("data", "legal", entry.File)
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var buf bytes.Buffer
		if err := md.Convert(raw, &buf); err != nil {
			return fmt.Errorf("render %s: %w", path, err)
		}
		legalSections[entry.Key] = template.HTML(buf.String())
	}
	return nil
}

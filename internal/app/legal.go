package app

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// legalSections holds rendered HTML by language and section, populated once
// at startup. Keeping legal copy in per-language Markdown files avoids a
// translated page shell around English-only policy text.
var legalSections = map[string]map[string]template.HTML{}

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
	for _, lang := range i18n.SupportedLangs {
		legalSections[lang] = map[string]template.HTML{}
		for _, entry := range legalFiles {
			filename := entry.File
			if lang != i18n.DefaultLang {
				ext := filepath.Ext(entry.File)
				filename = strings.TrimSuffix(entry.File, ext) + "." + lang + ext
			}
			path := filepath.Join("data", "legal", filename)
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			var buf bytes.Buffer
			if err := md.Convert(raw, &buf); err != nil {
				return fmt.Errorf("render %s: %w", path, err)
			}
			legalSections[lang][entry.Key] = template.HTML(buf.String())
		}
	}
	return nil
}

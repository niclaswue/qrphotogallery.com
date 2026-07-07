// Package i18n loads per-language string bundles from data/locales/*.json
// and resolves keys with fallback to the default language.
//
// URL convention: the default language has no prefix ("/foo"); every other
// language is served under "/<lang>/foo". This is a standard SEO setup —
// hreflang links published on each page tell search engines about the
// alternates. See LangPath() for the prefix used in templates.
package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// SupportedLangs is ordered: this is also the order shown in the language
// picker. Add a new lang here, drop a matching JSON file under data/locales/,
// add its name/locale below — routes, sitemap and hreflang pick it up
// automatically. The template ships with English (default) and German as a
// working demonstration of the pattern; trim to just "en" if your product
// launches single-language.
var SupportedLangs = []string{"en", "de"}

// DefaultLang is served at the root path with no language prefix.
const DefaultLang = "en"

// LangNames is the human-readable (autonym) name shown in the language picker.
var LangNames = map[string]string{
	"en": "English",
	"de": "Deutsch",
	"fr": "Français",
	"es": "Español",
	"it": "Italiano",
}

// LangLocales maps a lang code to a BCP-47 locale used for og:locale.
var LangLocales = map[string]string{
	"en": "en_US",
	"de": "de_DE",
	"fr": "fr_FR",
	"es": "es_ES",
	"it": "it_IT",
}

type Translations struct {
	mu   sync.RWMutex
	data map[string]map[string]string // lang -> key -> value
}

var global = &Translations{data: map[string]map[string]string{}}

// Load reads every <lang>.json file under dir into the package-global store.
// Missing files for declared langs are an error so we don't silently ship
// half-translated UI.
func Load(dir string) error {
	merged := map[string]map[string]string{}
	for _, lang := range SupportedLangs {
		path := filepath.Join(dir, lang+".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		merged[lang] = m
	}
	global.mu.Lock()
	global.data = merged
	global.mu.Unlock()
	return nil
}

// T returns the translation for key in lang, falling back to DefaultLang and
// finally to a bracketed key marker so missing copy is visible at a glance.
func T(lang, key string) string {
	global.mu.RLock()
	defer global.mu.RUnlock()
	if m, ok := global.data[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if lang != DefaultLang {
		if m, ok := global.data[DefaultLang]; ok {
			if v, ok := m[key]; ok {
				return v
			}
		}
	}
	return "[" + key + "]"
}

// IsSupported reports whether code is a supported site language.
func IsSupported(code string) bool {
	return slices.Contains(SupportedLangs, code)
}

// LangPath returns the URL prefix for a language. The default language has
// no prefix so the canonical URL is "/foo" rather than "/en/foo".
func LangPath(lang string) string {
	if lang == DefaultLang {
		return ""
	}
	return "/" + lang
}

// FromPath inspects the leading path segment and returns (lang, rest). If
// the leading segment is not a supported non-default language, lang is
// DefaultLang and rest is the original path.
func FromPath(path string) (lang, rest string) {
	trimmed := strings.TrimPrefix(path, "/")
	first, after, hasSlash := strings.Cut(trimmed, "/")
	if first != DefaultLang && IsSupported(first) {
		if hasSlash {
			return first, "/" + after
		}
		return first, "/"
	}
	return DefaultLang, path
}

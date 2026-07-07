package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// withRepoRoot chdirs to the repo root (where data/ and views/ live) for the
// duration of the test.
func withRepoRoot(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// loadLocale reads data/locales/<lang>.json into a key->value map. The caller
// must already be chdir'd to the repo root (see withRepoRoot).
func loadLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("data", "locales", lang+".json"))
	if err != nil {
		t.Fatalf("read locale %s: %v", lang, err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse locale %s: %v", lang, err)
	}
	return m
}

func keysOf(m map[string]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func sortedDiff(have, want map[string]bool) []string {
	var out []string
	for k := range want {
		if !have[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// TestLocaleParity asserts every language bundle carries exactly the English
// key set — no missing keys (which would silently fall back to English and
// ship untranslated UI to that locale) and no stray keys (typos / renames
// that never resolve). This is what keeps the whole site genuinely translated
// rather than half-English.
func TestLocaleParity(t *testing.T) {
	withRepoRoot(t)

	en := keysOf(loadLocale(t, i18n.DefaultLang))
	if len(en) == 0 {
		t.Fatalf("English locale bundle is empty")
	}

	for _, lang := range i18n.SupportedLangs {
		if lang == i18n.DefaultLang {
			continue
		}
		got := keysOf(loadLocale(t, lang))
		if missing := sortedDiff(got, en); len(missing) > 0 {
			t.Errorf("locale %s is missing %d key(s) present in en (would fall back to English): %v",
				lang, len(missing), missing)
		}
		if extra := sortedDiff(en, got); len(extra) > 0 {
			t.Errorf("locale %s has %d key(s) not present in en (dead/typo keys): %v",
				lang, len(extra), extra)
		}
	}
}

// TestFlagAssetsExist asserts every language we can show in the UI has a flag
// SVG on disk. The language switcher emits
// <img src="/static/img/flags/<lang>.svg"> for each language, so a missing
// file is a 404 in the browser.
func TestFlagAssetsExist(t *testing.T) {
	withRepoRoot(t)

	for _, lang := range i18n.SupportedLangs {
		path := filepath.Join("pb_public", "static", "img", "flags", lang+".svg")
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("missing flag asset for %q: %v", lang, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("flag asset for %q is empty: %s", lang, path)
		}
	}
}

// TestLangMetadataComplete asserts every supported language has a display
// name and an og:locale mapping, so the picker and social meta never render
// a blank.
func TestLangMetadataComplete(t *testing.T) {
	for _, lang := range i18n.SupportedLangs {
		if i18n.LangNames[lang] == "" {
			t.Errorf("i18n.LangNames missing an entry for %q", lang)
		}
		if i18n.LangLocales[lang] == "" {
			t.Errorf("i18n.LangLocales missing an entry for %q", lang)
		}
	}
}

package app

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

var referencedLocaleKey = regexp.MustCompile(`"((?:auth|cookie|create|edit|error|events|footer|forgot_password|guides|landing|legal|login|nav|overview|payment_success|poster|pricing|register|site|upload)\.[a-zA-Z0-9_.%]+)"`)

// TestReferencedLocaleKeysExist closes the gap left by parity alone: two
// locale files can be identically incomplete. Scan templates and handlers for
// every customer-facing key and require it in every language, so a literal
// [create.eyebrow] marker can never reach the browser again.
func TestReferencedLocaleKeysExist(t *testing.T) {
	withRepoRoot(t)

	bundles := make(map[string]map[string]string, len(i18n.SupportedLangs))
	for _, lang := range i18n.SupportedLangs {
		bundles[lang] = loadLocale(t, lang)
	}

	referenced := map[string]bool{}
	for _, root := range []string{"views", filepath.Join("internal", "app")} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || (filepath.Ext(path) != ".html" && filepath.Ext(path) != ".go") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, match := range referencedLocaleKey.FindAllSubmatch(raw, -1) {
				key := string(match[1])
				if strings.HasSuffix(key, ".typ") {
					continue
				}
				if strings.Contains(key, "%s") {
					for _, tier := range []string{"free", "standard", "premium"} {
						referenced[strings.ReplaceAll(key, "%s", tier)] = true
					}
					continue
				}
				referenced[key] = true
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan locale references: %v", err)
		}
	}

	for _, lang := range i18n.SupportedLangs {
		for key := range referenced {
			if _, ok := bundles[lang][key]; !ok {
				t.Errorf("locale %s is missing referenced key %q", lang, key)
			}
		}
	}
	for key := range bundles[i18n.DefaultLang] {
		if !referenced[key] {
			t.Errorf("locale key %q is no longer referenced; remove obsolete product copy", key)
		}
	}
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
// ship untranslated UI to that locale) and no language-specific extras. The
// reference scan above separately rejects dead product copy.
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

// TestLocalizedLegalFilesExist prevents a translated page shell from silently
// wrapping default-language legal content.
func TestLocalizedLegalFilesExist(t *testing.T) {
	withRepoRoot(t)

	for _, lang := range i18n.SupportedLangs {
		for _, entry := range legalFiles {
			filename := entry.File
			if lang != i18n.DefaultLang {
				ext := filepath.Ext(entry.File)
				filename = strings.TrimSuffix(entry.File, ext) + "." + lang + ext
			}
			path := filepath.Join("data", "legal", filename)
			info, err := os.Stat(path)
			if err != nil {
				t.Errorf("missing %s legal file for %q: %v", entry.Key, lang, err)
				continue
			}
			if info.Size() == 0 {
				t.Errorf("legal file for %q is empty: %s", lang, path)
			}
		}
	}
}

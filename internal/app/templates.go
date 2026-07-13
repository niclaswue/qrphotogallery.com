package app

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// pageTemplates caches parsed page templates keyed by page name. Each cache
// entry is the static HTML structure with a stub T/THTML that gets shadowed
// by per-request closures via Clone() on every render — see renderWithBase.
//
// Caching the parsed templates skips disk + parse cost on every request
// while still letting us bind per-request language. We reparse files when
// the cache misses, which on a first request is fine and during dev means a
// process restart shows new content immediately.
var (
	pageTemplatesMu sync.RWMutex
	pageTemplates   = map[string]*template.Template{}
)

// baseFuncs are the funcs that are safe to register at parse time because
// they don't depend on the current request. T/THTML are stubs here and get
// rebound per-request — see renderWithBase.
var baseFuncs = template.FuncMap{
	"add": func(a, b any) int {
		ai, _ := toInt(a)
		bi, _ := toInt(b)
		return ai + bi
	},
	"upper":     strings.ToUpper,
	"printf":    fmt.Sprintf,
	"hasLang":   i18n.IsSupported,
	"inStrings": func(list []string, item string) bool { return slices.Contains(list, item) },
	"T":         func(string) string { return "" },
	"THTML":     func(string) template.HTML { return "" },
}

func initTemplates() {
	// Loaded lazily by getPageTemplate; nothing to do up front.
}

func getPageTemplate(page string) (*template.Template, error) {
	pageTemplatesMu.RLock()
	if t, ok := pageTemplates[page]; ok {
		pageTemplatesMu.RUnlock()
		return t, nil
	}
	pageTemplatesMu.RUnlock()

	files := []string{
		"views/base.html",
		"views/_nav.html",
		"views/_footer.html",
		fmt.Sprintf("views/%s.html", page),
	}
	tmpl, err := template.New("base.html").Funcs(baseFuncs).ParseFiles(files...)
	if err != nil {
		return nil, err
	}
	pageTemplatesMu.Lock()
	pageTemplates[page] = tmpl
	pageTemplatesMu.Unlock()
	return tmpl, nil
}

// hreflangLink describes one alternate-language URL exposed via a
// <link rel="alternate"> element in the page head and the sitemap.
type hreflangLink struct {
	Lang string
	URL  string
}

// langSwitchOption represents one entry in the language picker. The Path
// is pre-built so the template doesn't have to concatenate strings inside
// an href attribute (Go's html/template forbids that as ambiguous URL
// context).
type langSwitchOption struct {
	Lang string
	Name string
	Path string
}

// renderWithBase renders a page that extends views/base.html. The request's
// language is derived from the URL prefix (i18n.FromPath); T/THTML are
// rebound on a fresh clone of the cached template so concurrent requests
// in different languages can't race.
func renderWithBase(e *core.RequestEvent, page string, data map[string]any) string {
	if data == nil {
		data = map[string]any{}
	}

	lang, langlessPath := i18n.FromPath(e.Request.URL.Path)
	if forced, ok := data["Lang"].(string); ok && i18n.IsSupported(forced) {
		lang = forced
	}
	if existing, ok := data["RequestPath"].(string); ok && existing != "" {
		langlessPath = existing
	}
	langPrefix := i18n.LangPath(lang)

	baseURL := strings.TrimRight(appConfig.AppURL, "/")
	links := make([]hreflangLink, 0, len(i18n.SupportedLangs))
	switchOptions := make([]langSwitchOption, 0, len(i18n.SupportedLangs))
	for _, l := range i18n.SupportedLangs {
		links = append(links, hreflangLink{
			Lang: l,
			URL:  baseURL + i18n.LangPath(l) + langlessPath,
		})
		// Each language link carries a ?setlang marker so picking one persists
		// the choice in the pref_lang cookie (see applyLangPreference) and the
		// visitor isn't bounced back to the default language on their next
		// bare-URL visit.
		switchOptions = append(switchOptions, langSwitchOption{
			Lang: l,
			Name: i18n.LangNames[l],
			Path: i18n.LangPath(l) + langlessPath + "?setlang=" + l,
		})
	}

	data["AppName"] = appConfig.AppName
	data["BaseURL"] = baseURL
	data["Year"] = time.Now().Year()
	data["Page"] = page
	data["BuildTime"] = BuildTime
	data["BuildCommit"] = BuildCommit
	data["Auth"] = e.Auth
	data["GoogleAuth"] = appConfig.GoogleOAuth.Enabled()
	data["Lang"] = lang
	data["LangPrefix"] = langPrefix
	data["LangPath"] = langlessPath
	data["LangLocale"] = i18n.LangLocales[lang]
	data["SupportedLangs"] = i18n.SupportedLangs
	data["LangNames"] = i18n.LangNames
	data["LangSwitchOptions"] = switchOptions
	data["HreflangLinks"] = links
	data["CanonicalURL"] = baseURL + langPrefix + langlessPath
	data["DefaultURL"] = baseURL + langlessPath
	if e.Auth != nil {
		data["UserName"] = e.Auth.GetString("name")
		if data["UserName"] == "" {
			data["UserName"] = e.Auth.Email()
		}
		data["UserTier"] = getUserTier(e.Auth)
	}

	if appConfig.PostHog.Key != "" {
		ph := map[string]string{
			"Key":       appConfig.PostHog.Key,
			"Host":      appConfig.PostHog.Host,
			"UserID":    "",
			"UserEmail": "",
			"UserTier":  "",
		}
		if e.Auth != nil {
			ph["UserID"] = e.Auth.Id
			ph["UserEmail"] = e.Auth.Email()
			ph["UserTier"] = getUserTier(e.Auth).Name
		}
		data["PostHog"] = ph
	}

	tmpl, err := getPageTemplate(page)
	if err != nil {
		log.Printf("Template parse error for %s: %v", page, err)
		return fmt.Sprintf("<html><body><h1>Error</h1><p>Failed to render page: %s</p><pre>%v</pre></body></html>", page, err)
	}

	clone, err := tmpl.Clone()
	if err != nil {
		return fmt.Sprintf("<html><body><pre>clone error: %v</pre></body></html>", err)
	}
	clone.Funcs(template.FuncMap{
		"T":     func(key string) string { return i18n.T(lang, key) },
		"THTML": func(key string) template.HTML { return template.HTML(i18n.T(lang, key)) },
	})

	var buf bytes.Buffer
	if err := clone.ExecuteTemplate(&buf, "base.html", data); err != nil {
		log.Printf("Template error for page %s: %v", page, err)
		return fmt.Sprintf("<html><body><h1>Error</h1><p>Failed to render page: %s</p><pre>%v</pre></body></html>", page, err)
	}
	return buf.String()
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		result, err := strconv.Atoi(n)
		if err != nil {
			return 0, false
		}
		return result, true
	default:
		return 0, false
	}
}

// redirectLocalised redirects to path while keeping the user in the
// language they came from. path must be language-less (e.g. "/overview");
// the helper prepends the current request's lang prefix so a German user
// stays on /de/overview rather than getting bounced to the default lang.
func redirectLocalised(e *core.RequestEvent, status int, path string) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	return e.Redirect(status, i18n.LangPath(lang)+path)
}

func renderHTMLError(e *core.RequestEvent, status int, title, message string) error {
	// message is trusted author content — we pass inline markup (e.g. the
	// "Upgrade for more" link on the plan-limit error) and want it rendered,
	// not escaped. Callers never put user input here.
	data := map[string]any{
		"ErrorTitle":   title,
		"ErrorMessage": template.HTML(message),
		"ShowLogin":    e.Auth == nil,
	}
	return e.HTML(status, renderWithBase(e, "error", data))
}

// renderHTMLErrorKeys resolves a customer-facing error in the request
// language. A ?lang= pin is honored for printed guest URLs before an event
// record is available; callers with a known event language can use the Lang
// variant below.
func renderHTMLErrorKeys(e *core.RequestEvent, status int, titleKey, messageKey string) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	if pinned := e.Request.URL.Query().Get("lang"); i18n.IsSupported(pinned) {
		lang = pinned
	}
	return renderHTMLErrorKeysLang(e, status, lang, titleKey, messageKey)
}

func renderHTMLErrorKeysLang(e *core.RequestEvent, status int, lang, titleKey, messageKey string) error {
	data := map[string]any{
		"Lang":         lang,
		"ErrorTitle":   i18n.T(lang, titleKey),
		"ErrorMessage": template.HTML(i18n.T(lang, messageKey)),
		"ShowLogin":    e.Auth == nil,
	}
	return e.HTML(status, renderWithBase(e, "error", data))
}

// renderQRDownloadLockedError shows an error page when a free-tier owner hits
// the paid-only bare QR downloads (PNG / ZIP), pushing them toward /pricing.
// The overview UI hides the download links for free owners, so this is the
// backstop for direct URL access.
func renderQRDownloadLockedError(e *core.RequestEvent) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	data := map[string]any{
		"ErrorTitle":   i18n.T(lang, "qr_download.locked.title"),
		"ErrorMessage": template.HTML(i18n.T(lang, "qr_download.locked.message")),
		"ShowLogin":    false,
		"PrimaryCTA": map[string]string{
			"Href":  i18n.LangPath(lang) + "/pricing",
			"Label": i18n.T(lang, "qr_download.locked.cta"),
		},
	}
	return e.HTML(http.StatusForbidden, renderWithBase(e, "error", data))
}

// renderUpgradeError shows an error page when the user exceeds their plan's
// prompt limit, pushing them toward /pricing.
func renderUpgradeError(e *core.RequestEvent, maxPrompts int) error {
	lang, _ := i18n.FromPath(e.Request.URL.Path)
	data := map[string]any{
		"ErrorTitle":   i18n.T(lang, "create.upgrade_error.title"),
		"ErrorMessage": template.HTML(fmt.Sprintf(i18n.T(lang, "create.upgrade_error.message"), maxPrompts)),
		"ShowLogin":    false,
		"PrimaryCTA": map[string]string{
			"Href":  i18n.LangPath(lang) + "/pricing",
			"Label": i18n.T(lang, "create.upgrade_error.cta"),
		},
	}
	return e.HTML(http.StatusOK, renderWithBase(e, "error", data))
}

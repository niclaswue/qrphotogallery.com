package app

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// pageTemplates caches one parsed template per page and language. Translation
// functions are bound before parsing, as required by html/template, so a page
// can never execute with the empty parser stubs that previously caused blank
// copy after partial reloads.
var (
	pageTemplatesMu sync.RWMutex
	pageTemplates   = map[string]*template.Template{}
)

// baseFuncs are request-independent helpers. Translation functions are added
// by getPageTemplate before each language-specific template is parsed.
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
}

func initTemplates() {
	// Loaded lazily by getPageTemplate; nothing to do up front.
}

func getPageTemplate(page, lang string) (*template.Template, error) {
	cacheKey := lang + ":" + page
	pageTemplatesMu.RLock()
	if t, ok := pageTemplates[cacheKey]; ok {
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
	funcs := template.FuncMap{}
	for name, fn := range baseFuncs {
		funcs[name] = fn
	}
	funcs["T"] = func(key string) string { return i18n.T(lang, key) }
	funcs["THTML"] = func(key string) template.HTML { return template.HTML(i18n.T(lang, key)) }
	tmpl, err := template.New("base.html").Funcs(funcs).ParseFiles(files...)
	if err != nil {
		return nil, err
	}
	pageTemplatesMu.Lock()
	pageTemplates[cacheKey] = tmpl
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
// already bound on the cached template for that language.
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

	tmpl, err := getPageTemplate(page, lang)
	if err != nil {
		log.Printf("Template parse error for %s: %v", page, err)
		return fmt.Sprintf("<html><body><h1>Error</h1><p>Failed to render page: %s</p><pre>%v</pre></body></html>", page, err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
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

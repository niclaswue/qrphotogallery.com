package app

import (
	"bufio"
	"log"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/osutils"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
	_ "github.com/niclaswue/template-qr-photo/migrations"
)

// appConfig is the process-wide config, populated once in Run before the
// router fires. Handler files read it directly.
var appConfig *Config

// BuildTime and BuildCommit are stamped at link time via -ldflags -X.
// Surfaced to every page so you can console-log it and verify which build
// served the response (i.e. whether you're hitting a stale cache).
var (
	BuildTime   = "dev"
	BuildCommit = "dev"
)

// Run starts the PocketBase app with all routes and bindings configured.
// It blocks until the server exits and fatal-logs on startup failure.
func Run() {
	app := pocketbase.New()

	loadDotEnv(".env")
	loadAppConfig()

	if err := i18n.Load("data/locales"); err != nil {
		log.Fatalf("could not load locales: %v", err)
	}

	if err := loadLegalContent(); err != nil {
		log.Fatalf("could not load legal content: %v", err)
	}

	if err := loadGuides(); err != nil {
		log.Fatalf("could not load guides: %v", err)
	}

	initTemplates()

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: osutils.IsProbablyGoRun(),
	})
	registerRetentionCleanup(app)

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		ensureSuperuser(e.App)
		return nil
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.BindFunc(attachAuthFromCookie)
		se.Router.BindFunc(applyLangPreference)
		registerRoutes(se)
		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// registerRoutes wires every public route. Handlers live in handlers_*.go
// files; this function exists purely to keep the route table readable.
//
// Localization: pages with SEO value are registered once per non-default
// language under the lang prefix (e.g. /de/pricing). The default language
// (en) keeps the bare path. Pages tied to a specific user/object — auth,
// payments, gallery, upload — are not localised in the URL because they're
// noindex anyway; the renderer still picks up the user's lang preference
// from the prefix when present.
func registerRoutes(se *core.ServeEvent) {
	r := se.Router

	// Public, indexable pages — duplicated per non-default language.
	registerLocalisedGet(r, "/", handleHome)
	registerLocalisedGet(r, "/pricing", handlePricing)
	registerLocalisedGet(r, "/legal", handleLegal)

	// SEO content hub. The index and every guide are localised and indexable;
	// they build topical authority around the product's core queries.
	registerLocalisedGet(r, "/guides", handleGuideIndex)
	registerLocalisedGet(r, "/guides/{slug}", handleGuide)

	// Host flow: create an event, then manage it from the overview.
	registerLocalisedGet(r, "/create", handleCreateStart)
	registerLocalisedPost(r, "/create", handleCreateSubmit)
	registerLocalisedGet(r, "/create/finish", handleCreateFinish)
	registerLocalisedGet(r, "/login", handleLoginPage)
	registerLocalisedPost(r, "/login", handleLoginSubmit)
	registerLocalisedGet(r, "/register", handleRegisterPage)
	registerLocalisedPost(r, "/register", handleRegisterSubmit)
	registerLocalisedGet(r, "/forgot-password", handleForgotPasswordPage)
	registerLocalisedPost(r, "/forgot-password", handleForgotPasswordSubmit)
	registerLocalisedGet(r, "/logout", handleLogout)

	// Google OAuth2. The start route is localised so each language's login
	// page can keep the user in their language after the round-trip; the
	// callback is a single canonical path because Google matches the
	// registered redirect URI exactly (the language is carried in a cookie).
	registerLocalisedGet(r, "/auth/google", handleGoogleLogin)
	r.GET("/auth/google/callback", handleGoogleCallback)

	registerLocalisedGet(r, "/overview", handleOverviewList)
	registerLocalisedGet(r, "/overview/{id}", handleOverview)
	registerLocalisedGet(r, "/gallery/{id}", handleLegacyOwnerGallery)
	registerLocalisedPost(r, "/upload/{id}/delete", handleDeleteUpload)
	registerLocalisedGet(r, "/poster/{id}", handlePrintPoster)
	registerLocalisedGet(r, "/qr-image/{id}", handleDownloadQRImage)
	registerLocalisedGet(r, "/download/{id}", handleDownloadGallery)

	registerLocalisedGet(r, "/edit/{id}", handleEditGallery)
	registerLocalisedPost(r, "/edit/{id}", handleEditGallerySubmit)
	registerLocalisedPost(r, "/settings/{id}", handleEventSettingsSubmit)
	registerLocalisedPost(r, "/language/{id}", handleEventLangSubmit)
	registerLocalisedPost(r, "/delete/{id}", handleDeleteEvent)

	// Guest flow — unauthenticated by design. Guests reach these by
	// scanning a printed QR code; requiring an account would kill the flow.
	registerLocalisedGet(r, "/e/{id}", handleEventDispatch)
	registerLocalisedUploadPost(r, "/e/{id}", handleEventDispatch)
	registerLocalisedGet(r, "/e/{id}/library", handleLegacyGuestGallery)
	registerLocalisedGet(r, "/e/{id}/download", handleGuestDownloadGallery)
	registerLocalisedGet(r, "/e/{id}/{promptID}", handleLegacyPromptLink)

	registerLocalisedGet(r, "/payment", handlePayment)
	registerLocalisedGet(r, "/payment/success", handlePaymentSuccess)

	// Webhooks and APIs are language-agnostic — they're never linked from
	// the UI, so a single mount at the canonical path is enough.
	r.POST("/webhook/lemon-squeezy", handleLemonWebhook)
	r.GET("/api/user/tier", handleGetUserTier).Bind(apis.RequireAuth())

	r.GET("/sitemap.xml", handleSitemap)
	r.GET("/robots.txt", handleRobots)
	r.GET("/.well-known/security.txt", handleSecurityTxt)
	r.GET("/security.txt", handleSecurityTxt)

	// AI / answer-engine descriptors. /llms.txt follows the llmstxt.org
	// convention: a concise, link-first Markdown map of the site for LLMs;
	// /llms-full.txt carries the same map with short descriptions inline.
	r.GET("/llms.txt", handleLLMsTxt)
	r.GET("/llms-full.txt", handleLLMsFullTxt)

	staticHandler := apis.Static(os.DirFS("./pb_public/static"), false)
	r.GET("/static/{path...}", func(e *core.RequestEvent) error {
		// Images, fonts and the favicon are versioned by filename and rarely
		// change, so cache them aggressively. CSS/JS may be updated in place
		// each deploy, so we keep a short max-age plus must-revalidate — long
		// enough to absorb a burst of page navigations, short enough that a
		// UI change picks up the new bundle within ~1 minute of the rollout.
		path := e.Request.URL.Path
		if strings.HasPrefix(path, "/static/img/") || strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".woff2") {
			e.Response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			e.Response.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
		}
		return staticHandler(e)
	})
}

// registerLocalisedGet registers a GET handler at path and at every
// non-default language prefix (e.g. /de/path). The default lang stays
// unprefixed so canonical URLs are clean.
func registerLocalisedGet(r *router.Router[*core.RequestEvent], path string, h func(*core.RequestEvent) error) {
	r.GET(path, h)
	for _, lang := range i18n.SupportedLangs {
		if lang == i18n.DefaultLang {
			continue
		}
		r.GET("/"+lang+path, h)
	}
}

func registerLocalisedPost(r *router.Router[*core.RequestEvent], path string, h func(*core.RequestEvent) error) {
	r.POST(path, h)
	for _, lang := range i18n.SupportedLangs {
		if lang == i18n.DefaultLang {
			continue
		}
		r.POST("/"+lang+path, h)
	}
}

// registerLocalisedUploadPost raises PocketBase's conservative 32 MB global
// request ceiling only for the guest media endpoint. Other forms keep the
// smaller default attack surface; upload batches may contain a 2 GB video or
// several originals up to the 4 GB request cap.
func registerLocalisedUploadPost(r *router.Router[*core.RequestEvent], path string, h func(*core.RequestEvent) error) {
	r.POST(path, h).Bind(apis.BodyLimit(maxUploadRequestSize))
	for _, lang := range i18n.SupportedLangs {
		if lang == i18n.DefaultLang {
			continue
		}
		r.POST("/"+lang+path, h).Bind(apis.BodyLimit(maxUploadRequestSize))
	}
}

// attachAuthFromCookie hydrates e.Auth from the pb_auth cookie when the
// PocketBase auth middleware hasn't already populated it (e.g. on plain
// HTML routes that don't use Bind(apis.RequireAuth())).
func attachAuthFromCookie(e *core.RequestEvent) error {
	if e.Auth == nil {
		if cookie, err := e.Request.Cookie("pb_auth"); err == nil && cookie.Value != "" {
			record, err := e.App.FindAuthRecordByToken(cookie.Value, core.TokenTypeAuth)
			if err == nil && record != nil {
				e.Auth = record
			}
		}
	}
	return e.Next()
}

// loadAppConfig populates the appConfig global from config.json (or the path
// in CONFIG_PATH) and overlays integration credentials from environment
// variables.
func loadAppConfig() {
	path := "config.json"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		path = envPath
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		log.Printf("Warning: could not load config from %s, using defaults: %v", path, err)
		cfg = defaultConfig()
	}

	cfg.LemonSqueezy = LemonSqueezyConfig{
		APIKey:        os.Getenv("LEMON_SQUEEZY_API_KEY"),
		StoreID:       os.Getenv("LEMON_SQUEEZY_STORE_ID"),
		WebhookSecret: os.Getenv("LEMON_SQUEEZY_WEBHOOK_SECRET"),
	}
	cfg.GoogleOAuth = GoogleOAuthConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	}
	if v := os.Getenv("POSTHOG_KEY"); v != "" {
		cfg.PostHog.Key = v
	}
	if v := os.Getenv("POSTHOG_HOST"); v != "" {
		cfg.PostHog.Host = v
	}
	if cfg.PostHog.Host == "" {
		cfg.PostHog.Host = "https://eu.i.posthog.com"
	}
	for i := range cfg.Tiers {
		envKey := "LEMON_SQUEEZY_PRODUCT_" + strings.ToUpper(cfg.Tiers[i].Name)
		if v := os.Getenv(envKey); v != "" {
			cfg.Tiers[i].LemonSqueezyVariantID = v
		}
	}
	appConfig = cfg
}

// loadDotEnv loads KEY=VALUE lines from path into the process env.
// Existing env vars take precedence, so production deploys can override.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

// ensureSuperuser creates a PocketBase superuser from PB_SUPERUSER_EMAIL /
// PB_SUPERUSER_PASSWORD if both are set and no matching account exists yet.
// Used to bootstrap admin access in fresh deployments.
func ensureSuperuser(app core.App) {
	email := os.Getenv("PB_SUPERUSER_EMAIL")
	password := os.Getenv("PB_SUPERUSER_PASSWORD")
	if email == "" || password == "" {
		return
	}

	existing, _ := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, email)
	if existing != nil {
		return
	}

	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		log.Printf("Warning: _superusers collection not found: %v", err)
		return
	}

	record := core.NewRecord(superusers)
	record.Set("email", email)
	record.Set("password", password)
	if err := app.Save(record); err != nil {
		log.Printf("Warning: failed to create superuser: %v", err)
		return
	}
	log.Printf("Created superuser: %s", email)
}

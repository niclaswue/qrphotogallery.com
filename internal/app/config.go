package app

import (
	"encoding/json"
	"os"
)

type TierConfig struct {
	Name       string `json:"name"`
	Price      string `json:"price"`
	PriceCents int    `json:"price_cents"`
	// LemonSqueezyVariantID is the variant ID used to create a checkout.
	// Loaded at startup from LEMON_SQUEEZY_PRODUCT_<NAME>.
	LemonSqueezyVariantID string `json:"lemon_squeezy_variant_id,omitempty"`
}

type LemonSqueezyConfig struct {
	APIKey        string
	StoreID       string
	WebhookSecret string
}

// GoogleOAuthConfig holds the Google OAuth2 client credentials. They come
// from env only (GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET) and are never
// serialised into config.json. When either is empty the "Continue with
// Google" button is hidden and the /auth/google routes refuse the flow, so
// the feature is fully opt-in per deployment.
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
}

// Enabled reports whether both Google OAuth credentials are present.
func (g GoogleOAuthConfig) Enabled() bool {
	return g.ClientID != "" && g.ClientSecret != ""
}

// PostHogConfig holds the public client-side PostHog credentials. The
// project API key is safe to ship to browsers (it's the `phc_...` token
// PostHog explicitly designs for inclusion in client JS); the host points
// at the regional ingestion endpoint (eu.i.posthog.com / us.i.posthog.com).
type PostHogConfig struct {
	Key  string `json:"key"`
	Host string `json:"host"`
}

type Config struct {
	AppName      string             `json:"app_name"`
	AppURL       string             `json:"app_url"`
	Tiers        []TierConfig       `json:"tiers"`
	S3           S3Config           `json:"s3"`
	FileStorage  string             `json:"file_storage"`
	PostHog      PostHogConfig      `json:"posthog"`
	LemonSqueezy LemonSqueezyConfig `json:"-"`
	GoogleOAuth  GoogleOAuthConfig  `json:"-"`
	// SupportEmail is the address surfaced as support contact on paid plans
	// and as the contact link in the imprint. Single source of truth so the
	// pricing CTA and the legal page can't drift.
	SupportEmail string `json:"support_email"`
}

type S3Config struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	Region   string `json:"region"`
	Access   string `json:"access_key,omitempty"`
	Secret   string `json:"secret_key,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return defaultConfig(), err
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return defaultConfig(), err
	}
	if len(cfg.Tiers) == 0 {
		cfg.Tiers = defaultConfig().Tiers
	}
	if cfg.SupportEmail == "" {
		cfg.SupportEmail = defaultConfig().SupportEmail
	}
	return &cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		AppName:      "QR Photo Gallery",
		AppURL:       "http://localhost:8090",
		FileStorage:  "local",
		SupportEmail: "hello@qrphotogallery.com",
		Tiers: []TierConfig{
			{Name: "free", Price: "Free", PriceCents: 0},
			{Name: "standard", Price: "€19", PriceCents: 1900},
			{Name: "premium", Price: "€29", PriceCents: 2900},
		},
	}
}

package app

import (
	"encoding/json"
	"os"
)

type TierConfig struct {
	Name       string `json:"name"`
	MaxPrompts int    `json:"max_prompts"`
	Price      string `json:"price"`
	PriceCents int    `json:"price_cents"`
	// LemonSqueezyVariantID is the variant ID used to create a checkout.
	// Loaded at startup from env vars (LEMON_SQUEEZY_PRODUCT_<NAME>) for
	// the control variant, or LEMON_SQUEEZY_PRODUCT_<VARIANT>_<NAME> for
	// non-control variants.
	LemonSqueezyVariantID string `json:"lemon_squeezy_variant_id,omitempty"`
}

// PricingVariant is one alternate pricing set, switched on by a PostHog
// feature flag (see pb_public/static/js/pricing-flags.js). With no flag on,
// the top-level Config.Tiers (the production default) are shown. Leave
// pricing_variants empty in config.json until you actually run a price test.
type PricingVariant struct {
	Name  string       `json:"name"`
	Tiers []TierConfig `json:"tiers"`
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
	// PricingVariants lists alternate pricing sets toggled by PostHog feature
	// flags. Defaults are the top-level Tiers.
	PricingVariants []PricingVariant `json:"pricing_variants,omitempty"`
	// SupportEmail is the address surfaced as support contact on paid plans
	// and as the contact link in the imprint. Single source of truth so the
	// pricing CTA and the legal page can't drift.
	SupportEmail string `json:"support_email"`
}

// DefaultVariantName is the implicit name used when no PostHog flag steers
// the visitor onto an alternate variant. It maps to Config.Tiers.
const DefaultVariantName = "default"

// pricingTiers returns the tier set for the named variant. Unknown or
// empty names fall back to the default (top-level) tier set.
func (c *Config) pricingTiers(variant string) []TierConfig {
	if variant == "" || variant == DefaultVariantName {
		return c.Tiers
	}
	for _, v := range c.PricingVariants {
		if v.Name == variant {
			return v.Tiers
		}
	}
	return c.Tiers
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
			{Name: "free", MaxPrompts: 1, Price: "Free", PriceCents: 0},
			{Name: "standard", MaxPrompts: 1, Price: "€19", PriceCents: 1900},
			{Name: "premium", MaxPrompts: 1, Price: "€29", PriceCents: 2900},
		},
	}
}

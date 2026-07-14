package app

import (
	"fmt"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/niclaswue/template-qr-photo/internal/i18n"
)

// publicSitemapEntry pairs an indexable path with its declared change
// priority. Each entry is the lang-less canonical path; the sitemap handler
// expands it into one <url> per supported language with hreflang alternates.
//
// Auth, gallery, upload, edit, payment, legal and webhook routes are
// intentionally excluded — they're either user-specific (no SEO value)
// or noindex'd already. Add content/guide pages here as you build them:
// content marketing pages are the main SEO lever for this kind of product.
type publicSitemapEntry struct {
	Path     string
	Priority string
}

var publicSitemapPaths = []publicSitemapEntry{
	{Path: "/", Priority: "1.0"},
	{Path: "/pricing", Priority: "0.9"},
	{Path: "/guides", Priority: "0.8"},
}

// sitemapEntries returns the static public paths plus one entry per guide,
// built from the guide registry so new content is indexed automatically.
func sitemapEntries() []publicSitemapEntry {
	entries := make([]publicSitemapEntry, 0, len(publicSitemapPaths)+len(guideSlugs))
	entries = append(entries, publicSitemapPaths...)
	for _, slug := range guideSlugs {
		priority := "0.7"
		if gc, ok := lookupGuide(slug, i18n.DefaultLang); ok && gc.Priority != "" {
			priority = gc.Priority
		}
		entries = append(entries, publicSitemapEntry{Path: "/guides/" + slug, Priority: priority})
	}
	return entries
}

// handleSitemap emits an XML sitemap with one <url> per (path, lang)
// pair. Every <url> includes <xhtml:link rel="alternate"> entries for
// each supported language and an x-default pointing at the bare path —
// this is Google's recommended pattern for multilingual sites.
func handleSitemap(e *core.RequestEvent) error {
	baseURL := strings.TrimRight(appConfig.AppURL, "/")
	lastmod := BuildTime
	if lastmod == "dev" || lastmod == "" {
		lastmod = "2026-01-01"
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	for _, entry := range sitemapEntries() {
		for _, lang := range i18n.SupportedLangs {
			sb.WriteString("  <url>\n")
			fmt.Fprintf(&sb, "    <loc>%s%s%s</loc>\n", baseURL, i18n.LangPath(lang), entry.Path)
			fmt.Fprintf(&sb, "    <lastmod>%s</lastmod>\n", lastmod)
			fmt.Fprintf(&sb, "    <priority>%s</priority>\n", entry.Priority)
			for _, alt := range i18n.SupportedLangs {
				fmt.Fprintf(&sb, "    <xhtml:link rel=\"alternate\" hreflang=\"%s\" href=\"%s%s%s\"/>\n",
					alt, baseURL, i18n.LangPath(alt), entry.Path)
			}
			fmt.Fprintf(&sb, "    <xhtml:link rel=\"alternate\" hreflang=\"x-default\" href=\"%s%s\"/>\n",
				baseURL, entry.Path)
			sb.WriteString("  </url>\n")
		}
	}
	sb.WriteString("</urlset>\n")

	e.Response.Header().Set("Content-Type", "application/xml; charset=utf-8")
	return e.String(200, sb.String())
}

// disallowedPaths are language-less route prefixes we don't want indexed:
// auth flows, user-specific pages, raw upload URLs, etc. handleRobots
// expands each one across every language prefix so e.g. /de/login is
// blocked alongside /login.
var disallowedPaths = []string{
	"/api/",
	"/webhook/",
	"/login",
	"/register",
	"/forgot-password",
	"/logout",
	"/auth/",
	"/overview",
	"/gallery/",
	"/print/",
	"/poster/",
	"/download/",
	"/edit/",
	"/e/",
	"/payment",
	"/_/",
	"/static/admin.html",
}

// welcomedAIAgents are AI / answer-engine crawlers we explicitly opt in.
// Getting cited by these engines is a core part of the SEO/AEO strategy, so
// rather than leaving them under the wildcard we name them and give them the
// same open-public / closed-private rules — a clear, auditable "yes, please
// read and cite our guides" signal. Removing a name here does NOT block that
// bot (the wildcard still applies); to block one, add an explicit Disallow.
var welcomedAIAgents = []string{
	"GPTBot", "OAI-SearchBot", "ChatGPT-User",
	"ClaudeBot", "Claude-SearchBot", "Claude-User",
	"PerplexityBot", "Perplexity-User",
	"Google-Extended", "Applebot-Extended",
}

// handleRobots emits the robots.txt. Public pages and guides are open; user,
// auth and upload paths are disallowed across every language prefix so search
// engines and AI crawlers don't index ephemeral or private URLs. We point at
// the sitemap and the /llms.txt site map for LLMs.
func handleRobots(e *core.RequestEvent) error {
	baseURL := strings.TrimRight(appConfig.AppURL, "/")

	var b strings.Builder
	b.WriteString("# QR Photo Gallery — robots.txt\n")
	b.WriteString("# Search engines and AI answer engines are welcome to crawl and cite our\n")
	b.WriteString("# public pages and guides. Private, per-user, auth and upload routes are\n")
	b.WriteString("# disallowed for everyone. LLM-friendly site map: " + baseURL + "/llms.txt\n\n")

	writeGroup := func(ua string) {
		b.WriteString("User-agent: " + ua + "\n")
		b.WriteString("Allow: /\n")
		for _, p := range disallowedPaths {
			b.WriteString("Disallow: " + p + "\n")
			for _, lang := range i18n.SupportedLangs {
				if lang == i18n.DefaultLang {
					continue
				}
				b.WriteString("Disallow: /" + lang + p + "\n")
			}
		}
		b.WriteString("\n")
	}

	writeGroup("*")
	for _, ua := range welcomedAIAgents {
		writeGroup(ua)
	}

	b.WriteString("Sitemap: " + baseURL + "/sitemap.xml\n")

	e.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	return e.String(200, b.String())
}

// handleLLMsTxt serves /llms.txt following the llmstxt.org convention: a
// concise, link-first Markdown map an LLM can read to understand the site and
// find the most useful pages. Guides are listed from the registry so the file
// stays in sync with the content that actually exists.
func handleLLMsTxt(e *core.RequestEvent) error {
	e.Response.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	return e.String(200, buildLLMsTxt(false))
}

// handleLLMsFullTxt serves /llms-full.txt: the same map with a short
// description under each link, for engines that fetch a single richer file.
func handleLLMsFullTxt(e *core.RequestEvent) error {
	e.Response.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	return e.String(200, buildLLMsTxt(true))
}

// buildLLMsTxt renders the /llms.txt (or /llms-full.txt when full is true)
// body. English is used as the canonical description language; German guide
// URLs are linked under a dedicated section so an engine can reach either.
func buildLLMsTxt(full bool) string {
	baseURL := strings.TrimRight(appConfig.AppURL, "/")
	var b strings.Builder

	b.WriteString("# " + appConfig.AppName + "\n\n")
	b.WriteString("> One printed QR code collects every photo and video from your event into a single shared gallery, in full original quality. Guests scan the code, upload from their phone browser — no app, no login — and the host downloads everything from one dashboard. One-time price, no subscription, EU-hosted.\n\n")
	b.WriteString("Best fit for weddings, prom/Abiball, birthdays, corporate events and graduations. Available in English and German.\n\n")

	b.WriteString("## Product\n\n")
	b.WriteString("- [Home](" + baseURL + "/): what it is and how the one-QR-code flow works")
	b.WriteString(descLine(full, "The main value proposition, how it works in three steps, and the live gallery example."))
	b.WriteString("- [Pricing](" + baseURL + "/pricing): one-time plans, no subscription")
	b.WriteString(descLine(full, "Standard and Commercial one-time plans, unlimited guests and uploads, EU hosting."))
	b.WriteString("- [Create a gallery](" + baseURL + "/create): start a new event gallery")
	b.WriteString(descLine(full, "The signup and event-creation flow for hosts."))
	b.WriteString("\n")

	b.WriteString("## Guides\n\n")
	for _, slug := range guideSlugs {
		gc, ok := lookupGuide(slug, i18n.DefaultLang)
		if !ok {
			continue
		}
		b.WriteString("- [" + gc.H1 + "](" + baseURL + "/guides/" + slug + ")")
		b.WriteString(descLine(full, gc.Description))
	}
	b.WriteString("\n")

	b.WriteString("## German (Deutsch)\n\n")
	b.WriteString("- [Startseite](" + baseURL + "/de/): QR-Code Fotogalerie für Events\n")
	b.WriteString("- [Ratgeber](" + baseURL + "/de/guides): alle Guides auf Deutsch\n")
	for _, slug := range guideSlugs {
		gc, ok := lookupGuide(slug, "de")
		if !ok {
			continue
		}
		b.WriteString("- [" + gc.H1 + "](" + baseURL + "/de/guides/" + slug + ")\n")
	}
	b.WriteString("\n")

	return b.String()
}

// descLine renders the trailing description for an llms.txt bullet: a short
// "— description" suffix in full mode, otherwise just the newline.
func descLine(full bool, desc string) string {
	if full && desc != "" {
		return "\n  " + desc + "\n"
	}
	return "\n"
}

// handleSecurityTxt serves /.well-known/security.txt per RFC 9116 so
// researchers know where to disclose vulnerabilities.
func handleSecurityTxt(e *core.RequestEvent) error {
	baseURL := strings.TrimRight(appConfig.AppURL, "/")
	body := strings.Join([]string{
		"Contact: mailto:" + appConfig.SupportEmail,
		"Preferred-Languages: en, de",
		"Canonical: " + baseURL + "/.well-known/security.txt",
		"Policy: " + baseURL + "/legal",
		"",
	}, "\n")
	e.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	return e.String(200, body)
}

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
	for _, entry := range publicSitemapPaths {
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

// handleRobots emits the robots.txt. We disallow user/auth/upload paths
// across every language prefix so search engines don't index ephemeral
// or private URLs, and we point at the sitemap.
func handleRobots(e *core.RequestEvent) error {
	baseURL := strings.TrimRight(appConfig.AppURL, "/")
	lines := []string{"User-agent: *", "Allow: /"}
	for _, p := range disallowedPaths {
		lines = append(lines, "Disallow: "+p)
		for _, lang := range i18n.SupportedLangs {
			if lang == i18n.DefaultLang {
				continue
			}
			lines = append(lines, "Disallow: /"+lang+p)
		}
	}
	lines = append(lines, "", "Sitemap: "+baseURL+"/sitemap.xml", "")
	e.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	return e.String(200, strings.Join(lines, "\n"))
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

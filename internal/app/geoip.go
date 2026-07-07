package app

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// lookupCountryByIP resolves an IPv4/IPv6 address to an ISO 3166-1 alpha-2
// country code via ipapi.co. The free tier serves 30k requests/month with
// no API key and supports HTTPS — plenty for signup-time lookups on a
// small SaaS.
//
// Returns "" on any failure (private/loopback IP, network error, rate
// limit, unparseable response). Callers should treat the empty string as
// "unknown" and not block the signup path on it.
func lookupCountryByIP(ctx context.Context, ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" || isLocalIP(ip) {
		return ""
	}

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	url := "https://ipapi.co/" + ip + "/country/"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	// ipapi.co rejects requests without a UA on some edges.
	req.Header.Set("User-Agent", "qr-photo-app/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return ""
	}
	code := strings.ToUpper(strings.TrimSpace(string(body)))
	if !countryCodeRE.MatchString(code) {
		return ""
	}
	return code
}

var countryCodeRE = regexp.MustCompile(`^[A-Z]{2}$`)

// isLocalIP returns true for loopback, link-local, and RFC1918 private
// ranges so we don't waste a network round-trip on traffic that
// obviously won't resolve to a country.
func isLocalIP(ip string) bool {
	switch {
	case ip == "127.0.0.1", ip == "::1", ip == "0.0.0.0":
		return true
	case strings.HasPrefix(ip, "10."),
		strings.HasPrefix(ip, "192.168."),
		strings.HasPrefix(ip, "169.254."),
		strings.HasPrefix(ip, "fc00:"),
		strings.HasPrefix(ip, "fd"),
		strings.HasPrefix(ip, "fe80:"):
		return true
	}
	// 172.16.0.0 – 172.31.255.255
	if strings.HasPrefix(ip, "172.") {
		var second int
		for i, b := range []byte(ip)[4:] {
			if b == '.' {
				if i == 0 {
					return false
				}
				break
			}
			if b < '0' || b > '9' {
				return false
			}
			second = second*10 + int(b-'0')
		}
		if second >= 16 && second <= 31 {
			return true
		}
	}
	return false
}

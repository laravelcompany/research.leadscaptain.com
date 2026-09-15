// Package webcheck fetches a company website and extracts the signals a lead
// researcher cares about: whether the site is alive, its title and meta
// description, and cheap technology fingerprints. It needs no API key.
package webcheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Result struct {
	Domain       string   `json:"domain"`
	URL          string   `json:"url"`
	FinalURL     string   `json:"final_url"`
	Reachable    bool     `json:"reachable"`
	StatusCode   int      `json:"status_code"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Generator    string   `json:"generator"`
	Server       string   `json:"server"`
	PoweredBy    string   `json:"powered_by"`
	Technologies []string `json:"technologies"`
	Error        string   `json:"error,omitempty"`
}

var (
	titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	descRe  = regexp.MustCompile(`(?is)<meta[^>]+name=["']description["'][^>]+content=["']([^"']*)["']`)
	descRe2 = regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']*)["'][^>]+name=["']description["']`)
	genRe   = regexp.MustCompile(`(?is)<meta[^>]+name=["']generator["'][^>]+content=["']([^"']*)["']`)
	tagRe   = regexp.MustCompile(`<[^>]+>`)
	spaceRe = regexp.MustCompile(`\s+`)
)

// fingerprints maps a technology name to substrings that reliably indicate it
// in page HTML or response headers. Matching is case-insensitive.
var fingerprints = []struct {
	Name  string
	Hints []string
}{
	{"WordPress", []string{"wp-content/", "wp-includes/"}},
	{"Shopify", []string{"cdn.shopify.com", "shopify.theme"}},
	{"Wix", []string{"wixstatic.com", "X-Wix-"}},
	{"Squarespace", []string{"squarespace.com/universal/scripts", "static1.squarespace.com"}},
	{"Laravel", []string{"laravel_session", "XSRF-TOKEN"}},
	{"Next.js", []string{"__NEXT_DATA__", "/_next/static/"}},
	{"React", []string{"react-dom", "data-reactroot", "__react"}},
	{"Vue.js", []string{"vue.js", "vue.min.js", "__vue__"}},
	{"Cloudflare", []string{"cloudflare"}},
	{"HubSpot", []string{"hs-scripts.com", "hubspot"}},
	{"Intercom", []string{"intercomcdn", "widget.intercom.io"}},
	{"Stripe", []string{"js.stripe.com"}},
	{"Google Analytics", []string{"googletagmanager.com", "google-analytics.com"}},
	{"jQuery", []string{"jquery"}},
}

// Check fetches https://domain (falling back to http) and extracts signals.
func Check(ctx context.Context, domain string) Result {
	domain = strings.TrimSpace(strings.ToLower(domain))
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	if i := strings.IndexAny(domain, "/?#"); i >= 0 {
		domain = domain[:i]
	}
	res := Result{Domain: domain}
	if domain == "" || !strings.Contains(domain, ".") {
		res.Error = "invalid domain"
		return res
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	var resp *http.Response
	var err error
	for _, scheme := range []string{"https", "http"} {
		res.URL = scheme + "://" + domain
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, "GET", res.URL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; lead-research/1.0)")
		resp, err = client.Do(req)
		if err == nil {
			break
		}
	}
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()
	res.Reachable = true
	res.StatusCode = resp.StatusCode
	res.FinalURL = resp.Request.URL.String()
	res.Server = resp.Header.Get("Server")
	res.PoweredBy = resp.Header.Get("X-Powered-By")
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	html := string(body)
	if m := titleRe.FindStringSubmatch(html); m != nil {
		res.Title = clean(m[1])
	}
	if m := descRe.FindStringSubmatch(html); m != nil {
		res.Description = clean(m[1])
	} else if m := descRe2.FindStringSubmatch(html); m != nil {
		res.Description = clean(m[1])
	}
	if m := genRe.FindStringSubmatch(html); m != nil {
		res.Generator = clean(m[1])
	}
	haystack := strings.ToLower(html + " " + res.Server + " " + res.PoweredBy)
	seen := map[string]bool{}
	for _, fp := range fingerprints {
		for _, h := range fp.Hints {
			if strings.Contains(haystack, strings.ToLower(h)) && !seen[fp.Name] {
				res.Technologies = append(res.Technologies, fp.Name)
				seen[fp.Name] = true
			}
		}
	}
	return res
}

func clean(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	s = spaceRe.ReplaceAllString(strings.TrimSpace(s), " ")
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

package webcheck

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Analysis is the full Websites-section report for one URL: SEO metrics,
// page speed, scraped contacts, keyword density and a 0-100 health score.
// Everything comes from fetching the site itself - no paid API. Traffic
// estimates are not available from any free source; TrafficNote says so
// instead of inventing numbers.
type Analysis struct {
	URL             string       `json:"url"`
	Domain          string       `json:"domain"`
	FinalURL        string       `json:"final_url,omitempty"`
	Reachable       bool         `json:"reachable"`
	StatusCode      int          `json:"status_code,omitempty"`
	LoadMs          int64        `json:"load_ms"`
	Title           string       `json:"title,omitempty"`
	MetaDescription string       `json:"meta_description,omitempty"`
	H1Tags          []string     `json:"h1_tags,omitempty"`
	Emails          []string     `json:"emails,omitempty"`
	Phones          []string     `json:"phones,omitempty"`
	Socials         SocialLinks  `json:"socials"`
	Keywords        []Keyword    `json:"keywords,omitempty"`
	Checks          []ScoreCheck `json:"checks"`
	HealthScore     int          `json:"health_score"`
	TrafficNote     string       `json:"traffic_note"`
	Error           string       `json:"error,omitempty"`
}

// Keyword is one term with its frequency in visible page text.
type Keyword struct {
	Word    string  `json:"word"`
	Count   int     `json:"count"`
	Density float64 `json:"density"` // percent of total words
}

// Check is one scored SEO rule in the health score breakdown.
type ScoreCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Points int    `json:"points"` // points earned (0 when failed)
	Max    int    `json:"max"`
	Detail string `json:"detail,omitempty"`
}

var (
	h1Re        = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	viewportRe  = regexp.MustCompile(`(?is)<meta[^>]+name=["']viewport["']`)
	canonicalRe = regexp.MustCompile(`(?is)<link[^>]+rel=["']canonical["']`)
	langRe      = regexp.MustCompile(`(?is)<html[^>]+lang=["']([a-zA-Z-]+)["']`)
	mailtoRe    = regexp.MustCompile(`(?i)mailto:([a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,})`)
	emailRe     = regexp.MustCompile(`(?i)\b([a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,})\b`)
	telRe       = regexp.MustCompile(`(?i)tel:([+0-9 ().\-]{6,20})`)
	phoneRe     = regexp.MustCompile(`(\+\d{1,3}[ .\-]?\(?\d{2,4}\)?[ .\-]?\d{3,4}[ .\-]?\d{3,4})`)
	scriptRe    = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
	wordRe      = regexp.MustCompile(`[a-zA-Z][a-zA-Z'\-]{2,}`)
	imgAltRe    = regexp.MustCompile(`(?is)<img[^>]*>`)
	altAttrRe   = regexp.MustCompile(`(?is)alt=["'][^"']+["']`)
)

// stopwords are excluded from keyword density.
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "you": true, "your": true,
	"are": true, "our": true, "not": true, "all": true, "can": true, "has": true,
	"have": true, "from": true, "that": true, "this": true, "will": true, "was": true,
	"were": true, "been": true, "more": true, "about": true, "into": true, "their": true,
	"they": true, "them": true, "who": true, "what": true, "when": true, "where": true,
	"which": true, "how": true, "out": true, "use": true, "new": true, "get": true,
	"its": true, "but": true, "one": true, "two": true, "also": true, "than": true,
	"then": true, "now": true, "here": true, "there": true, "www": true, "com": true,
	"home": true, "contact": true, "privacy": true, "terms": true, "cookie": true,
	"cookies": true, "menu": true, "close": true, "open": true, "read": true, "skip": true,
	"content": true, "copyright": true, "reserved": true, "rights": true,
}

// Analyze fetches the homepage (plus likely contact pages) of rawURL and
// builds the report. It is deterministic and side-effect free.
func Analyze(ctx context.Context, rawURL string) Analysis {
	norm := normalizeURL(rawURL)
	a := Analysis{URL: norm, TrafficNote: "traffic estimates need a paid data provider; not available from free sources"}
	if norm == "" {
		a.Error = "invalid url"
		return a
	}
	if i := strings.IndexAny(strings.TrimPrefix(strings.TrimPrefix(norm, "https://"), "http://"), "/?#"); i >= 0 {
		host := strings.TrimPrefix(strings.TrimPrefix(norm, "https://"), "http://")
		a.Domain = host[:i]
	} else {
		a.Domain = strings.TrimPrefix(strings.TrimPrefix(norm, "https://"), "http://")
	}

	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	html, status, finalURL, loadMs, err := fetchPage(ctx, client, norm)
	if err != nil {
		a.Error = err.Error()
		return a
	}
	a.Reachable = true
	a.StatusCode = status
	a.FinalURL = finalURL
	a.LoadMs = loadMs

	a.Title = firstMatch(titleRe, html)
	a.MetaDescription = firstMatch(descRe, html)
	if a.MetaDescription == "" {
		a.MetaDescription = firstMatch(descRe2, html)
	}
	for _, m := range h1Re.FindAllStringSubmatch(html, -1) {
		if t := clean(m[1]); t != "" {
			a.H1Tags = append(a.H1Tags, t)
		}
	}

	// Contact scraping: homepage first, then likely contact pages (same host).
	emails, phones := extractContacts(html)
	socials := ExtractSocials(html)
	for _, path := range []string{"/contact", "/contact-us", "/about", "/about-us"} {
		if len(emails) >= 2 && len(phones) >= 1 {
			break
		}
		page := strings.TrimSuffix(finalURL, "/") + path
		sub, st, _, _, err := fetchPage(ctx, client, page)
		if err != nil || st >= 400 {
			continue
		}
		e2, p2 := extractContacts(sub)
		emails = appendUnique(emails, e2...)
		phones = appendUnique(phones, p2...)
		if s2 := ExtractSocials(sub); s2 != (SocialLinks{}) {
			socials = mergeSocials(socials, s2)
		}
	}
	a.Emails, a.Phones, a.Socials = emails, phones, socials
	a.Keywords = topKeywords(html, 10)
	a.Checks = scoreChecks(a, html, norm)
	total := 0
	for _, c := range a.Checks {
		total += c.Points
	}
	a.HealthScore = total
	return a
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	return raw
}

func fetchPage(ctx context.Context, client *http.Client, url string) (string, int, string, int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", 0, "", 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; lead-research/1.0)")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", 0, err
	}
	defer resp.Body.Close()
	load := time.Since(start).Milliseconds()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	return string(body), resp.StatusCode, resp.Request.URL.String(), load, nil
}

func firstMatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return clean(m[1])
	}
	return ""
}

func extractContacts(html string) ([]string, []string) {
	var emails, phones []string
	seenE, seenP := map[string]bool{}, map[string]bool{}
	add := func(list *[]string, seen map[string]bool, v string) {
		v = strings.TrimSpace(strings.Trim(v, ".,;:)]\"'"))
		low := strings.ToLower(v)
		if v == "" || seen[low] {
			return
		}
		seen[low] = true
		*list = append(*list, v)
	}
	for _, m := range mailtoRe.FindAllStringSubmatch(html, -1) {
		add(&emails, seenE, m[1])
	}
	// Strip mailto: matches already collected, then plain-text emails.
	for _, m := range emailRe.FindAllStringSubmatch(html, -1) {
		e := m[1]
		if strings.HasSuffix(strings.ToLower(e), ".png") || strings.HasSuffix(strings.ToLower(e), ".jpg") || strings.Contains(e, "@2x") {
			continue
		}
		add(&emails, seenE, e)
	}
	for _, m := range telRe.FindAllStringSubmatch(html, -1) {
		add(&phones, seenP, m[1])
	}
	if len(phones) == 0 {
		for _, m := range phoneRe.FindAllStringSubmatch(html, -1) {
			add(&phones, seenP, m[1])
			if len(phones) >= 3 {
				break
			}
		}
	}
	return emails, phones
}

func appendUnique(list []string, more ...string) []string {
	seen := map[string]bool{}
	for _, v := range list {
		seen[strings.ToLower(v)] = true
	}
	for _, v := range more {
		if !seen[strings.ToLower(v)] {
			seen[strings.ToLower(v)] = true
			list = append(list, v)
		}
	}
	return list
}

func mergeSocials(a, b SocialLinks) SocialLinks {
	if a.LinkedIn == "" {
		a.LinkedIn = b.LinkedIn
	}
	if a.Twitter == "" {
		a.Twitter = b.Twitter
	}
	if a.Facebook == "" {
		a.Facebook = b.Facebook
	}
	if a.Crunchbase == "" {
		a.Crunchbase = b.Crunchbase
	}
	return a
}

// topKeywords counts content words in visible text and returns the top n by
// frequency with density percentages.
func topKeywords(html string, n int) []Keyword {
	text := scriptRe.ReplaceAllString(html, " ")
	text = tagRe.ReplaceAllString(text, " ")
	words := wordRe.FindAllString(strings.ToLower(text), -1)
	counts := map[string]int{}
	total := 0
	for _, w := range words {
		w = strings.Trim(w, "'-")
		if len(w) < 3 || stopwords[w] {
			continue
		}
		counts[w]++
		total++
	}
	kws := make([]Keyword, 0, len(counts))
	for w, c := range counts {
		if c < 2 {
			continue
		}
		kws = append(kws, Keyword{Word: w, Count: c})
	}
	sort.Slice(kws, func(i, j int) bool {
		if kws[i].Count != kws[j].Count {
			return kws[i].Count > kws[j].Count
		}
		return kws[i].Word < kws[j].Word
	})
	if len(kws) > n {
		kws = kws[:n]
	}
	for i := range kws {
		if total > 0 {
			kws[i].Density = float64(kws[i].Count) / float64(total) * 100
		}
	}
	return kws
}

// scoreChecks grades the page against fixed SEO rules; the health score is
// the sum of earned points (max 100), so the UI can show the breakdown.
func scoreChecks(a Analysis, html, requestedURL string) []ScoreCheck {
	checks := []ScoreCheck{}
	add := func(name string, max int, passed bool, detail string) {
		pts := 0
		if passed {
			pts = max
		}
		checks = append(checks, ScoreCheck{Name: name, Passed: passed, Points: pts, Max: max, Detail: detail})
	}
	add("Title present", 10, a.Title != "", a.Title)
	tl := len(a.Title)
	add("Title length 10-60 chars", 5, tl >= 10 && tl <= 60, fmt.Sprintf("%d chars", tl))
	add("Meta description present", 10, a.MetaDescription != "", a.MetaDescription)
	dl := len(a.MetaDescription)
	add("Description length 50-160 chars", 5, dl >= 50 && dl <= 160, fmt.Sprintf("%d chars", dl))
	add("Exactly one H1", 10, len(a.H1Tags) == 1, fmt.Sprintf("%d found", len(a.H1Tags)))
	add("HTTPS", 10, strings.HasPrefix(a.FinalURL, "https://") || strings.HasPrefix(requestedURL, "https://"), a.FinalURL)
	add("Viewport meta (mobile)", 10, viewportRe.MatchString(html), "")
	add("Load under 3s", 10, a.LoadMs < 3000, fmt.Sprintf("%d ms", a.LoadMs))
	add("Contact info on site", 10, len(a.Emails)+len(a.Phones) > 0, "")
	add("Canonical link", 5, canonicalRe.MatchString(html), "")
	lang := langRe.FindStringSubmatch(html)
	add("HTML lang attribute", 5, lang != nil, "")
	imgs := imgAltRe.FindAllString(html, -1)
	withAlt := 0
	for _, img := range imgs {
		if altAttrRe.MatchString(img) {
			withAlt++
		}
	}
	add("Images have alt text", 10, len(imgs) == 0 || withAlt*2 >= len(imgs), fmt.Sprintf("%d/%d", withAlt, len(imgs)))
	return checks
}

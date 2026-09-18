package webcheck

import (
	"regexp"
	"strings"
)

// SocialLinks are the company social profiles discoverable on its homepage.
type SocialLinks struct {
	LinkedIn   string `json:"linkedin,omitempty"`
	Twitter    string `json:"twitter,omitempty"`
	Facebook   string `json:"facebook,omitempty"`
	Crunchbase string `json:"crunchbase,omitempty"`
}

var socialRe = regexp.MustCompile(`(?i)href=["'](https?://(?:[a-z0-9-]+\.)?(?:linkedin\.com|twitter\.com|x\.com|facebook\.com|crunchbase\.com)[^"']*)["']`)

// ExtractSocials pulls LinkedIn/Twitter(X)/Facebook/Crunchbase links out of
// page HTML. Footer and share-widget links to other people's profiles are the
// main false positives; the first company-looking link wins per network.
func ExtractSocials(html string) SocialLinks {
	var out SocialLinks
	seen := map[string]bool{}
	for _, m := range socialRe.FindAllStringSubmatch(html, -1) {
		u := strings.TrimSpace(m[1])
		lower := strings.ToLower(u)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		switch {
		case strings.Contains(lower, "linkedin.com"):
			// Share-intent and help/login pages are not the company profile.
			if strings.Contains(lower, "/share") || strings.Contains(lower, "/help") || out.LinkedIn != "" {
				continue
			}
			out.LinkedIn = u
		case strings.Contains(lower, "twitter.com"), strings.Contains(lower, "x.com"):
			if strings.Contains(lower, "/share") || strings.Contains(lower, "intent") || out.Twitter != "" {
				continue
			}
			out.Twitter = u
		case strings.Contains(lower, "facebook.com"):
			if strings.Contains(lower, "sharer") || out.Facebook != "" {
				continue
			}
			out.Facebook = u
		case strings.Contains(lower, "crunchbase.com"):
			if out.Crunchbase == "" {
				out.Crunchbase = u
			}
		}
	}
	return out
}

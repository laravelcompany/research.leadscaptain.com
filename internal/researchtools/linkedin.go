package researchtools

import (
	"fmt"
	"regexp"
	"strings"
)

// LinkedInFormat standardizes a LinkedIn profile or company link for
// database entry: canonical host, /in/<slug> or /company/<slug>, no tracking
// parameters, no trailing slash.
type LinkedInFormat struct {
	Input     string `json:"input"`
	Formatted string `json:"formatted,omitempty"`
	Kind      string `json:"kind,omitempty"` // profile | company | school | unknown
	Slug      string `json:"slug,omitempty"`
	Error     string `json:"error,omitempty"`
}

var (
	linkedinSlugRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9\-_%]{1,99}$`)
	linkedinPathRe = regexp.MustCompile(`(?i)^/?(in|company|school)/([^/?#]+)`)
)

// FormatLinkedIn normalizes one URL or bare slug. It accepts full URLs in any
// scheme/host form, "in/<slug>", or a bare profile slug.
func FormatLinkedIn(input string) LinkedInFormat {
	out := LinkedInFormat{Input: input}
	s := strings.TrimSpace(input)
	if s == "" {
		out.Error = "input is required"
		return out
	}
	// Strip scheme and host variants.
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	for _, host := range []string{"www.linkedin.com", "linkedin.com", "uk.linkedin.com", "www.linkedin.co.uk", "linkedin.co.uk"} {
		if strings.HasPrefix(strings.ToLower(s), host) {
			s = s[len(host):]
			break
		}
	}
	// Drop query and fragment (tracking parameters).
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, "/")

	kind, slug := "", ""
	if m := linkedinPathRe.FindStringSubmatch(s); m != nil {
		kind, slug = strings.ToLower(m[1]), m[2]
	} else if !strings.ContainsAny(s, "/") {
		kind, slug = "in", s // bare handle defaults to a personal profile
	} else {
		out.Error = "not a recognizable LinkedIn profile or company link"
		return out
	}
	if !linkedinSlugRe.MatchString(slug) {
		out.Error = fmt.Sprintf("invalid LinkedIn slug %q", slug)
		return out
	}
	out.Kind = map[string]string{"in": "profile", "company": "company", "school": "school"}[kind]
	out.Slug = slug
	out.Formatted = "https://www.linkedin.com/" + kind + "/" + slug
	return out
}

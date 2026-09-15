package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"research-leads/internal/emailvalidator"
)

// FindEmailTool generates the common corporate email patterns for a person at
// a domain and verifies each candidate with the configured email validator,
// returning the candidates with their verdicts. Free when the validator is
// the built-in mock or a free-tier provider.
type FindEmailTool struct{ ev *emailvalidator.Client }

func NewFindEmail(ev *emailvalidator.Client) *FindEmailTool { return &FindEmailTool{ev: ev} }
func (t *FindEmailTool) Name() string                       { return "find_email" }
func (t *FindEmailTool) Description() string {
	return "Guess and verify email patterns for a person: {\"first_name\":\"Ada\",\"last_name\":\"Lovelace\",\"domain\":\"acme.com\"}. Returns candidates with verification status."
}

var nonAlpha = regexp.MustCompile(`[^a-z]`)

// Patterns returns the candidate addresses in order of corporate likelihood.
func Patterns(first, last, domain string) []string {
	f := nonAlpha.ReplaceAllString(strings.ToLower(first), "")
	l := nonAlpha.ReplaceAllString(strings.ToLower(last), "")
	d := strings.TrimSpace(strings.ToLower(domain))
	if f == "" || d == "" || !strings.Contains(d, ".") {
		return nil
	}
	fi := f[:1]
	if l == "" {
		return []string{f + "@" + d}
	}
	return []string{
		f + "." + l + "@" + d,
		f + "@" + d,
		fi + l + "@" + d,
		f + l + "@" + d,
		fi + "." + l + "@" + d,
		f + "_" + l + "@" + d,
	}
}

type Candidate struct {
	Email  string `json:"email"`
	Status string `json:"status"`
}

func (t *FindEmailTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var p struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Domain    string `json:"domain"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return nil, err
	}
	cands := Patterns(p.FirstName, p.LastName, p.Domain)
	if len(cands) == 0 {
		return nil, fmt.Errorf("find_email needs first_name and a valid domain")
	}
	out := []Candidate{}
	best := ""
	for _, c := range cands {
		r, err := t.ev.Verify(ctx, c)
		st := "unknown"
		if err == nil && r.Status != "" {
			st = r.Status
		}
		out = append(out, Candidate{Email: c, Status: st})
		if best == "" && st == "valid" {
			best = c
		}
	}
	return map[string]any{"domain": p.Domain, "candidates": out, "best": best}, nil
}

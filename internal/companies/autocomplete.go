package companies

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Suggestion is one type-ahead match for the company search box.
type Suggestion struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Logo   string `json:"logo,omitempty"`
}

// AutocompleteClient wraps the free, keyless Clearbit autocomplete endpoint.
// It suggests well-known companies by partial name or domain. Failures are
// soft: the caller treats an error as "no suggestions", never as fatal.
type AutocompleteClient struct {
	http    *http.Client
	baseURL string
}

func NewAutocomplete(timeout int) *AutocompleteClient {
	if timeout <= 0 {
		timeout = 15
	}
	return &AutocompleteClient{
		http:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
		baseURL: "https://autocomplete.clearbit.com/v1/companies/suggest",
	}
}

// Suggest returns up to limit suggestions for a partial company name or
// domain. An unreachable or erroring service yields an empty slice and a nil
// error: suggestions are a convenience, not a failure mode.
func (c *AutocompleteClient) Suggest(ctx context.Context, query string, limit int) ([]Suggestion, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"?query="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var raw []struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
		Logo   string `json:"logo"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, nil
	}
	out := make([]Suggestion, 0, len(raw))
	for _, r := range raw {
		if r.Name == "" && r.Domain == "" {
			continue
		}
		out = append(out, Suggestion{Name: r.Name, Domain: r.Domain, Logo: r.Logo})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

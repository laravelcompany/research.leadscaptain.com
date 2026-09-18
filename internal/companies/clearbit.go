package companies

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClearbitClient enriches a domain with firmographic data from the Clearbit
// Company API. It is OPTIONAL: the zero-cost default build works without it
// (registry + website signals only). Set CLEARBIT_API_KEY to enable.
type ClearbitClient struct {
	apiKey  string
	http    *http.Client
	baseURL string
}

// NewClearbit returns nil when no key is configured, so callers can test for
// nil and skip enrichment entirely.
func NewClearbit(apiKey string, timeout int) *ClearbitClient {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 15
	}
	return &ClearbitClient{
		apiKey:  apiKey,
		http:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
		baseURL: "https://company.clearbit.com/v2/companies/find",
	}
}

// Enrichment is the subset of Clearbit company fields the profile view shows.
type Enrichment struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Industry    string `json:"industry,omitempty"`
	Employees   string `json:"employees,omitempty"`
	Revenue     string `json:"revenue,omitempty"`
	FoundedYear string `json:"founded_year,omitempty"`
	HQLocation  string `json:"hq_location,omitempty"`
	LinkedIn    string `json:"linkedin,omitempty"`
	Twitter     string `json:"twitter,omitempty"`
	Facebook    string `json:"facebook,omitempty"`
	Crunchbase  string `json:"crunchbase,omitempty"`
}

// Enrich looks up one domain. A 404 (unknown domain) is not an error, just an
// empty result; transport and auth failures are returned as errors so the
// caller can note them honestly.
func (c *ClearbitClient) Enrich(ctx context.Context, domain string) (*Enrichment, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"?domain="+url.QueryEscape(domain), nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.apiKey, "")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clearbit status %d", resp.StatusCode)
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		FoundedYear int    `json:"foundedYear"`
		Category    struct {
			Industry string `json:"industry"`
		} `json:"category"`
		Metrics struct {
			Employees     int    `json:"employees"`
			EmployeeRange string `json:"employeesRange"`
			AnnualRevenue int64  `json:"annualRevenue"`
		} `json:"metrics"`
		Geo struct {
			City    string `json:"city"`
			Country string `json:"country"`
		} `json:"geo"`
		LinkedIn struct {
			Handle string `json:"handle"`
		} `json:"linkedin"`
		Twitter struct {
			Handle string `json:"handle"`
		} `json:"twitter"`
		Facebook struct {
			Handle string `json:"handle"`
		} `json:"facebook"`
		Crunchbase struct {
			Handle string `json:"handle"`
		} `json:"crunchbase"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("clearbit decode: %w", err)
	}
	e := &Enrichment{
		Name:        body.Name,
		Description: body.Description,
		Industry:    body.Category.Industry,
	}
	if body.Metrics.EmployeeRange != "" {
		e.Employees = body.Metrics.EmployeeRange
	} else if body.Metrics.Employees > 0 {
		e.Employees = fmt.Sprintf("%d", body.Metrics.Employees)
	}
	if body.Metrics.AnnualRevenue > 0 {
		e.Revenue = fmt.Sprintf("$%d", body.Metrics.AnnualRevenue)
	}
	if body.FoundedYear > 0 {
		e.FoundedYear = fmt.Sprintf("%d", body.FoundedYear)
	}
	e.HQLocation = strings.Trim(strings.TrimSpace(body.Geo.City+", "+body.Geo.Country), ", ")
	if body.LinkedIn.Handle != "" {
		e.LinkedIn = "https://linkedin.com/" + strings.TrimPrefix(body.LinkedIn.Handle, "/")
	}
	if body.Twitter.Handle != "" {
		e.Twitter = "https://twitter.com/" + strings.TrimPrefix(body.Twitter.Handle, "/")
	}
	if body.Facebook.Handle != "" {
		e.Facebook = "https://facebook.com/" + strings.TrimPrefix(body.Facebook.Handle, "/")
	}
	if body.Crunchbase.Handle != "" {
		e.Crunchbase = "https://crunchbase.com/" + strings.TrimPrefix(body.Crunchbase.Handle, "/")
	}
	return e, nil
}

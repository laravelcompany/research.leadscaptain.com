package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DomainAge is the RDAP-sourced registration report for one domain.
type DomainAge struct {
	Domain     string `json:"domain"`
	Created    string `json:"created,omitempty"`
	Expires    string `json:"expires,omitempty"`
	Updated    string `json:"updated,omitempty"`
	Registrar  string `json:"registrar,omitempty"`
	AgeDisplay string `json:"age_display,omitempty"`
	Source     string `json:"source"`
	Error      string `json:"error,omitempty"`
}

// DomainAgeChecker queries RDAP (rdap.org bootstrap), the free, keyless,
// registry-run successor to WHOIS.
type DomainAgeChecker struct {
	http    *http.Client
	baseURL string
}

func NewDomainAgeChecker(timeout int) *DomainAgeChecker {
	if timeout <= 0 {
		timeout = 15
	}
	return &DomainAgeChecker{
		http:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
		baseURL: "https://rdap.org/domain/",
	}
}

// rdapResponse models the bits of RFC 9083 JSON we read.
type rdapResponse struct {
	Events []struct {
		Action string `json:"eventAction"`
		Date   string `json:"eventDate"`
	} `json:"events"`
	Entities []struct {
		Roles      []string        `json:"roles"`
		VcardArray json.RawMessage `json:"vcardArray"`
	} `json:"entities"`
}

// Check resolves creation/expiry dates and registrar for a domain.
func (c *DomainAgeChecker) Check(ctx context.Context, domain string) DomainAge {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")
	if i := strings.IndexAny(domain, "/?#"); i >= 0 {
		domain = domain[:i]
	}
	out := DomainAge{Domain: domain, Source: "rdap.org"}
	if domain == "" || !strings.Contains(domain, ".") {
		out.Error = "invalid domain"
		return out
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+domain, nil)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		out.Error = "domain not found in registry"
		return out
	}
	if resp.StatusCode != http.StatusOK {
		out.Error = fmt.Sprintf("rdap status %d", resp.StatusCode)
		return out
	}
	var body rdapResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		out.Error = "could not parse rdap response"
		return out
	}
	for _, e := range body.Events {
		date := e.Date
		if len(date) > 10 {
			date = date[:10]
		}
		switch e.Action {
		case "registration":
			out.Created = date
		case "expiration":
			out.Expires = date
		case "last changed", "last update", "last modified":
			out.Updated = date
		}
	}
	for _, ent := range body.Entities {
		isRegistrar := false
		for _, r := range ent.Roles {
			if r == "registrar" {
				isRegistrar = true
			}
		}
		if !isRegistrar {
			continue
		}
		// vcardArray: ["vcard", [["version",{},"text","4.0"],["fn",{},"text","Registrar Ltd"],...]]
		var vcard []json.RawMessage
		if err := json.Unmarshal(ent.VcardArray, &vcard); err != nil || len(vcard) < 2 {
			continue
		}
		var props [][]json.RawMessage
		if err := json.Unmarshal(vcard[1], &props); err != nil {
			continue
		}
		for _, prop := range props {
			if len(prop) < 4 {
				continue
			}
			var name, val string
			json.Unmarshal(prop[0], &name)
			json.Unmarshal(prop[3], &val)
			if name == "fn" && val != "" {
				out.Registrar = val
				break
			}
		}
	}
	if out.Created != "" {
		out.AgeDisplay = humanAge(out.Created)
	}
	return out
}

// humanAge renders "8 years 3 months" from a YYYY-MM-DD date.
func humanAge(created string) string {
	t, err := time.Parse("2006-01-02", created)
	if err != nil {
		return ""
	}
	now := time.Now()
	years := now.Year() - t.Year()
	months := int(now.Month()) - int(t.Month())
	if months < 0 {
		years--
		months += 12
	}
	if now.Day() < t.Day() {
		months--
		if months < 0 {
			years--
			months += 12
		}
	}
	if years <= 0 && months <= 0 {
		return "under a month"
	}
	if years <= 0 {
		return fmt.Sprintf("%d months", months)
	}
	return fmt.Sprintf("%d years %d months", years, months)
}

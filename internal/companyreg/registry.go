// Package companyreg verifies companies against genuinely free European
// public registries, routed by country:
//
//	GB  - Companies House Public Data API (free key, name search)
//	FR  - recherche-entreprises.api.gouv.fr (no key, name search)
//	RO  - ANAF PlatitorTvaRest (no key, by fiscal code / CUI)
//	EU* - VIES VAT validation (no key, by VAT number; EU-27 + XI)
//
// Every route fails safe with a clear, actionable error when its input or
// configuration is missing; unsupported name searches say so instead of
// pretending to have checked.
package companyreg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	CHKey   string
	Timeout int
	http    *http.Client
	// base URLs are fields so tests can point each provider at httptest servers.
	chURL   string
	viesURL string
	frURL   string
	roURL   string
}

func New(chKey string, timeout int) *Client {
	if timeout <= 0 {
		timeout = 30
	}
	return &Client{
		CHKey:   chKey,
		Timeout: timeout,
		http:    &http.Client{Timeout: time.Duration(timeout) * time.Second},
		chURL:   "https://api.company-information.service.gov.uk",
		viesURL: "https://ec.europa.eu/taxation_customs/vies/rest-api",
		frURL:   "https://recherche-entreprises.api.gouv.fr",
		roURL:   "https://webservicesp.anaf.ro/PlatitorTvaRest/api/v8/ws/tva",
	}
}

// Query is one lookup request. VAT number wins over everything (it is the
// strongest, fully-free identifier), then country routes the provider.
type Query struct {
	Name       string `json:"name"`
	Country    string `json:"country"`
	VATNumber  string `json:"vat_number"`
	FiscalCode string `json:"fiscal_code"` // RO CUI
}

type Company struct {
	Name      string `json:"name"`
	Number    string `json:"number,omitempty"`
	Status    string `json:"status,omitempty"`
	Type      string `json:"type,omitempty"`
	Created   string `json:"created,omitempty"`
	Address   string `json:"address,omitempty"`
	VATNumber string `json:"vat_number,omitempty"`
	VATValid  *bool  `json:"vat_valid,omitempty"`
}

type Result struct {
	Provider  string    `json:"provider"`
	Country   string    `json:"country"`
	Total     int       `json:"total_results,omitempty"`
	Companies []Company `json:"companies"`
	Note      string    `json:"note,omitempty"`
}

// viesCountries are the member states VIES validates (EU-27 plus XI for
// Northern Ireland). GB is deliberately absent post-Brexit.
var viesCountries = map[string]bool{
	"AT": true, "BE": true, "BG": true, "CY": true, "CZ": true, "DE": true,
	"DK": true, "EE": true, "EL": true, "ES": true, "FI": true, "FR": true,
	"HR": true, "HU": true, "IE": true, "IT": true, "LT": true, "LU": true,
	"LV": true, "MT": true, "NL": true, "PL": true, "PT": true, "RO": true,
	"SE": true, "SK": true, "SI": true, "XI": true,
}

// splitVAT splits "DE123456789" into ("DE", "123456789"). A bare number is
// returned with an empty country so the caller can apply the query country.
func splitVAT(vat string) (cc, num string) {
	vat = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(vat), " ", ""))
	if len(vat) > 2 && vat[0] >= 'A' && vat[0] <= 'Z' && vat[1] >= 'A' && vat[1] <= 'Z' && !(vat[2] >= 'A' && vat[2] <= 'Z') {
		return vat[:2], vat[2:]
	}
	return "", vat
}

// Lookup routes the query to the right free provider.
func (c *Client) Lookup(ctx context.Context, q Query) (Result, error) {
	country := strings.ToUpper(strings.TrimSpace(q.Country))
	if country == "UK" {
		country = "GB"
	}
	if q.VATNumber != "" {
		vcc, num := splitVAT(q.VATNumber)
		if vcc == "" {
			vcc = country
		}
		if vcc == "" {
			return Result{}, fmt.Errorf("vat_number %q has no country prefix; pass country as well", q.VATNumber)
		}
		if !viesCountries[vcc] {
			return Result{}, fmt.Errorf("VIES does not cover %s (EU-27 + XI only); use country-specific lookup", vcc)
		}
		return c.vies(ctx, vcc, num)
	}
	switch country {
	case "GB":
		if q.Name == "" {
			return Result{}, fmt.Errorf("company_lookup GB needs name")
		}
		return c.companiesHouse(ctx, q.Name)
	case "FR":
		if q.Name == "" {
			return Result{}, fmt.Errorf("company_lookup FR needs name")
		}
		return c.france(ctx, q.Name)
	case "RO":
		if q.FiscalCode == "" {
			return Result{}, fmt.Errorf("Romania's free ANAF registry searches by fiscal code (CUI), not name; pass fiscal_code")
		}
		return c.romania(ctx, q.FiscalCode)
	case "":
		return Result{}, fmt.Errorf("company_lookup needs country (GB/FR/RO supported for name or code search) or vat_number (any EU state)")
	default:
		if viesCountries[country] {
			return Result{}, fmt.Errorf("no free name-search registry wired for %s; pass vat_number to validate via VIES instead", country)
		}
		return Result{}, fmt.Errorf("unsupported country %q; supported: GB, FR, RO (name/code search) or any EU VAT number via VIES", country)
	}
}

func (c *Client) get(ctx context.Context, url string, basicUser string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if basicUser != "" {
		req.SetBasicAuth(basicUser, "")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("registry status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

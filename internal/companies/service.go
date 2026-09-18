// Package companies powers the Companies section: search across free sources
// (keyless autocomplete + official European registries), a merged profile view
// (registry firmographics + website signals + optional Clearbit enrichment),
// CSV export, and converting a company result into a lead.
//
// Cost policy: the default build spends nothing. Clearbit autocomplete and the
// country registries are free; CLEARBIT_API_KEY enables paid-style enrichment
// and everything degrades safely without it.
package companies

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"research-leads/internal/companyreg"
	"research-leads/internal/webcheck"
)

// Service orchestrates the free data sources and the cache table.
type Service struct {
	DB           *sql.DB
	Autocomplete *AutocompleteClient
	Clearbit     *ClearbitClient // nil unless CLEARBIT_API_KEY is set
	Registry     *companyreg.Client
	// CheckSite fetches website signals; defaults to webcheck.Check. A field
	// so tests can stub the network.
	CheckSite func(ctx context.Context, domain string) webcheck.Result
}

// SearchQuery is the Companies search bar: one free-text box plus the
// optional filters the spec asks for.
type SearchQuery struct {
	Query    string `json:"query"`
	Country  string `json:"country"`
	Industry string `json:"industry"`
	Location string `json:"location"`
}

// SearchResult is one merged row in the results list.
type SearchResult struct {
	Name           string `json:"name"`
	Domain         string `json:"domain,omitempty"`
	Source         string `json:"source"` // autocomplete | registry | cache
	RegistryNumber string `json:"registry_number,omitempty"`
	Status         string `json:"status,omitempty"`
	Address        string `json:"address,omitempty"`
	CachedID       int64  `json:"cached_id,omitempty"`
}

// SearchResponse carries the merged results plus any honest limitation notes
// (e.g. a registry that needed a key).
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Notes   []string       `json:"notes,omitempty"`
}

// Profile is the full company view assembled from every free source.
type Profile struct {
	Name            string   `json:"name"`
	Domain          string   `json:"domain,omitempty"`
	Description     string   `json:"description,omitempty"`
	Industry        string   `json:"industry,omitempty"`
	EmployeeCount   string   `json:"employee_count,omitempty"`
	FoundedYear     string   `json:"founded_year,omitempty"`
	HQLocation      string   `json:"hq_location,omitempty"`
	RevenueEstimate string   `json:"revenue_estimate,omitempty"`
	LinkedIn        string   `json:"linkedin_url,omitempty"`
	Twitter         string   `json:"twitter_url,omitempty"`
	Facebook        string   `json:"facebook_url,omitempty"`
	Crunchbase      string   `json:"crunchbase_url,omitempty"`
	TechStack       []string `json:"tech_stack,omitempty"`
	RegistryNumber  string   `json:"registry_number,omitempty"`
	RegistryStatus  string   `json:"registry_status,omitempty"`
	RegistryCountry string   `json:"registry_country,omitempty"`
	Sources         []string `json:"sources"`
	Notes           []string `json:"notes,omitempty"`
}

func (s *Service) checkSite(ctx context.Context, domain string) webcheck.Result {
	if s.CheckSite != nil {
		return s.CheckSite(ctx, domain)
	}
	return webcheck.Check(ctx, domain)
}

// looksLikeDomain detects "acme.com" vs "acme".
func looksLikeDomain(q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	return strings.Contains(q, ".") && !strings.ContainsAny(q, " /?#@")
}

// countryFromDomain infers a registry country from a ccTLD we support.
func countryFromDomain(domain string) string {
	switch {
	case strings.HasSuffix(domain, ".uk"):
		return "GB"
	case strings.HasSuffix(domain, ".fr"):
		return "FR"
	case strings.HasSuffix(domain, ".ro"):
		return "RO"
	}
	return ""
}

// Search merges keyless autocomplete, official registries and the local cache
// into one ranked list. Industry/location filters apply as post-filters on
// sources that carry that data; a note says so when a filter could not be
// applied anywhere.
func (s *Service) Search(ctx context.Context, q SearchQuery) SearchResponse {
	q.Query = strings.TrimSpace(q.Query)
	resp := SearchResponse{Results: []SearchResult{}}
	seen := map[string]bool{}
	add := func(r SearchResult) {
		key := strings.ToLower(r.Domain + "|" + r.Name)
		if key == "|" || seen[key] {
			return
		}
		seen[key] = true
		resp.Results = append(resp.Results, r)
	}

	if s.Autocomplete != nil {
		if sug, err := s.Autocomplete.Suggest(ctx, q.Query, 10); err == nil {
			for _, g := range sug {
				add(SearchResult{Name: g.Name, Domain: g.Domain, Source: "autocomplete"})
			}
		}
	}

	if s.Registry != nil && q.Query != "" && !looksLikeDomain(q.Query) {
		country := strings.ToUpper(strings.TrimSpace(q.Country))
		if country == "UK" {
			country = "GB"
		}
		countries := []string{}
		switch country {
		case "":
			countries = []string{"FR", "GB"} // FR is keyless; GB notes when its free key is missing
		case "FR", "GB":
			countries = []string{country}
		default:
			resp.Notes = append(resp.Notes, fmt.Sprintf("name search is not available for %s; RO and EU lookups need a fiscal or VAT number", country))
		}
		for _, cc := range countries {
			res, err := s.Registry.Lookup(ctx, companyreg.Query{Name: q.Query, Country: cc})
			if err != nil {
				resp.Notes = append(resp.Notes, err.Error())
				continue
			}
			for _, c := range res.Companies {
				add(SearchResult{
					Name:           c.Name,
					Source:         "registry",
					RegistryNumber: c.Number,
					Status:         c.Status,
					Address:        c.Address,
				})
			}
			if res.Note != "" {
				resp.Notes = append(resp.Notes, res.Note)
			}
		}
	}

	if s.DB != nil && q.Query != "" {
		like := "%" + q.Query + "%"
		rows, err := s.DB.Query("SELECT id,name,domain FROM companies WHERE name LIKE ? OR domain LIKE ? ORDER BY updated_at DESC LIMIT 10", like, like)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var name, dom sql.NullString
				if rows.Scan(&id, &name, &dom) == nil {
					add(SearchResult{Name: name.String, Domain: dom.String, Source: "cache", CachedID: id})
				}
			}
		}
	}

	// Post-filters for sources that expose the field. Registry results carry
	// addresses; industry is only known on cached/enriched rows.
	if loc := strings.ToLower(strings.TrimSpace(q.Location)); loc != "" {
		filtered := resp.Results[:0]
		for _, r := range resp.Results {
			if r.Source == "registry" && !strings.Contains(strings.ToLower(r.Address), loc) {
				continue
			}
			filtered = append(filtered, r)
		}
		resp.Results = filtered
	}
	if strings.TrimSpace(q.Industry) != "" {
		resp.Notes = append(resp.Notes, "industry filter applies after opening a profile; free registries do not expose industry in search")
	}
	return resp
}

// Profile assembles the company view for one domain or name: registry
// firmographics, website description/tech/socials, optional Clearbit
// enrichment, and the cache table is upserted so repeat views are free.
func (s *Service) Profile(ctx context.Context, domain, name, country string) (Profile, error) {
	p := Profile{Sources: []string{}}
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")
	if i := strings.IndexAny(domain, "/?#"); i >= 0 {
		domain = domain[:i]
	}
	name = strings.TrimSpace(name)
	if domain == "" && name == "" {
		return p, fmt.Errorf("domain or name is required")
	}
	p.Domain = domain
	p.Name = name

	if domain != "" {
		site := s.checkSite(ctx, domain)
		if site.Reachable {
			p.Sources = append(p.Sources, "website")
			if p.Name == "" {
				p.Name = site.Title
			}
			p.Description = site.Description
			p.TechStack = site.Technologies
			p.LinkedIn = site.Socials.LinkedIn
			p.Twitter = site.Socials.Twitter
			p.Facebook = site.Socials.Facebook
			p.Crunchbase = site.Socials.Crunchbase
		} else if site.Error != "" {
			p.Notes = append(p.Notes, "website not reachable: "+site.Error)
		}
	}

	if country == "" && domain != "" {
		country = countryFromDomain(domain)
	}
	if s.Registry != nil && name != "" {
		cc := strings.ToUpper(country)
		if cc == "UK" {
			cc = "GB"
		}
		if cc == "" {
			cc = "FR" // keyless fallback when nothing points at a country
		}
		if cc == "FR" || cc == "GB" {
			if res, err := s.Registry.Lookup(ctx, companyreg.Query{Name: name, Country: cc}); err == nil && len(res.Companies) > 0 {
				c := res.Companies[0]
				p.Sources = append(p.Sources, "registry")
				if p.Name == "" || looksLikeDomain(p.Name) {
					p.Name = c.Name
				}
				p.RegistryNumber = c.Number
				p.RegistryStatus = c.Status
				p.RegistryCountry = cc
				p.HQLocation = c.Address
				if len(c.Created) >= 4 {
					p.FoundedYear = c.Created[:4]
				}
			}
		}
	}

	if s.Clearbit != nil && domain != "" {
		if e, err := s.Clearbit.Enrich(ctx, domain); err != nil {
			p.Notes = append(p.Notes, "clearbit enrichment failed: "+err.Error())
		} else if e != nil {
			p.Sources = append(p.Sources, "clearbit")
			overlayProfile(&p, e)
		}
	} else if domain != "" {
		p.Notes = append(p.Notes, "employee count and revenue need CLEARBIT_API_KEY; not available from free sources")
	}

	if s.DB != nil && p.Name != "" {
		s.upsert(ctx, p)
	}
	return p, nil
}

// overlayProfile fills gaps from Clearbit enrichment without discarding
// registry/website values already collected.
func overlayProfile(p *Profile, e *Enrichment) {
	if p.Name == "" {
		p.Name = e.Name
	}
	if p.Description == "" {
		p.Description = e.Description
	}
	if p.Industry == "" {
		p.Industry = e.Industry
	}
	if p.EmployeeCount == "" {
		p.EmployeeCount = e.Employees
	}
	if p.RevenueEstimate == "" {
		p.RevenueEstimate = e.Revenue
	}
	if p.FoundedYear == "" {
		p.FoundedYear = e.FoundedYear
	}
	if p.HQLocation == "" {
		p.HQLocation = e.HQLocation
	}
	if p.LinkedIn == "" {
		p.LinkedIn = e.LinkedIn
	}
	if p.Twitter == "" {
		p.Twitter = e.Twitter
	}
	if p.Facebook == "" {
		p.Facebook = e.Facebook
	}
	if p.Crunchbase == "" {
		p.Crunchbase = e.Crunchbase
	}
}

// upsert caches the profile keyed on domain (or by name when no domain).
func (s *Service) upsert(ctx context.Context, p Profile) {
	tech, _ := json.Marshal(p.TechStack)
	raw, _ := json.Marshal(p)
	// The companies table predates this section (001_init.sql) with an
	// INTEGER employee_count; numeric headcounts go there, ranges ("51-200")
	// go to employee_range.
	empInt, empRange := splitEmployees(p.EmployeeCount)
	cols := "name,industry,employee_count,employee_range,founded_year,hq_location,revenue_estimate,description,linkedin_url,twitter_url,facebook_url,crunchbase_url,tech_stack,source,registry_number,registry_status,registry_country,raw_data"
	vals := []any{p.Name, p.Industry, empInt, empRange, p.FoundedYear, p.HQLocation, p.RevenueEstimate, p.Description, p.LinkedIn, p.Twitter, p.Facebook, p.Crunchbase, string(tech), strings.Join(p.Sources, ","), p.RegistryNumber, p.RegistryStatus, p.RegistryCountry, string(raw)}
	if p.Domain != "" {
		args := append([]any{p.Domain, websiteURL(p.Domain)}, vals...)
		s.DB.ExecContext(ctx, `INSERT INTO companies(domain,website,`+cols+`,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
			ON CONFLICT(domain) DO UPDATE SET name=excluded.name,industry=excluded.industry,employee_count=excluded.employee_count,employee_range=excluded.employee_range,founded_year=excluded.founded_year,hq_location=excluded.hq_location,revenue_estimate=excluded.revenue_estimate,description=excluded.description,linkedin_url=excluded.linkedin_url,twitter_url=excluded.twitter_url,facebook_url=excluded.facebook_url,crunchbase_url=excluded.crunchbase_url,tech_stack=excluded.tech_stack,source=excluded.source,registry_number=excluded.registry_number,registry_status=excluded.registry_status,registry_country=excluded.registry_country,raw_data=excluded.raw_data,updated_at=CURRENT_TIMESTAMP`, args...)
		return
	}
	args := append(vals, p.Name)
	s.DB.ExecContext(ctx, `INSERT INTO companies(`+cols+`,updated_at)
		SELECT ?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP WHERE NOT EXISTS (SELECT 1 FROM companies WHERE name=? AND domain IS NULL)`, args...)
}

// SaveToLead converts a cached company (or an ad-hoc name+domain) into a
// lead row, deduped on company_domain. Returns the new lead id, or the
// existing lead id when the domain was already saved.
func (s *Service) SaveToLead(ctx context.Context, companyID int64, name, domain string) (int64, bool, error) {
	if s.DB == nil {
		return 0, false, fmt.Errorf("database unavailable")
	}
	if companyID > 0 {
		var n, d sql.NullString
		var ind, cc, city sql.NullString
		err := s.DB.QueryRowContext(ctx, "SELECT name,domain,industry,registry_country,hq_location FROM companies WHERE id=?", companyID).Scan(&n, &d, &ind, &cc, &city)
		if err != nil {
			return 0, false, fmt.Errorf("company not found")
		}
		name, domain = n.String, d.String
		res, err := s.DB.ExecContext(ctx, `INSERT INTO leads(company_name,company_domain,website_url,industry_name,country_code,city,source,email_status)
			SELECT ?,?,?,?,?,?,?,'unchecked' WHERE NOT EXISTS (SELECT 1 FROM leads WHERE company_domain=? AND company_domain<>'')`,
			name, domain, websiteURL(domain), ind.String, cc.String, city.String, "company_search", domain)
		if err != nil {
			return 0, false, err
		}
		return leadIDAfterInsert(ctx, s.DB, res, domain)
	}
	name = strings.TrimSpace(name)
	domain = strings.ToLower(strings.TrimSpace(domain))
	if name == "" && domain == "" {
		return 0, false, fmt.Errorf("company_id or name/domain is required")
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO leads(company_name,company_domain,website_url,source,email_status)
		SELECT ?,?,?,?,'unchecked' WHERE NOT EXISTS (SELECT 1 FROM leads WHERE company_domain=? AND company_domain<>'')`,
		name, domain, websiteURL(domain), "company_search", domain)
	if err != nil {
		return 0, false, err
	}
	return leadIDAfterInsert(ctx, s.DB, res, domain)
}

// splitEmployees routes a headcount to the INTEGER employee_count column or
// the TEXT employee_range column added by 008_companies.sql.
func splitEmployees(v string) (any, any) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return nil, v
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func websiteURL(domain string) string {
	if domain == "" {
		return ""
	}
	return "https://" + domain
}

func leadIDAfterInsert(ctx context.Context, db *sql.DB, res sql.Result, domain string) (int64, bool, error) {
	if n, _ := res.RowsAffected(); n > 0 {
		id, _ := res.LastInsertId()
		return id, true, nil
	}
	var id int64
	err := db.QueryRowContext(ctx, "SELECT id FROM leads WHERE company_domain=? ORDER BY id DESC LIMIT 1", domain).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("lead already exists for this domain")
	}
	return id, false, nil
}

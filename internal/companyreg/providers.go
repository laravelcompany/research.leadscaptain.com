package companyreg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// companiesHouse searches the UK Companies House Public Data API. Free key,
// 600 requests per 5 minutes; key goes in the basic-auth username.
func (c *Client) companiesHouse(ctx context.Context, name string) (Result, error) {
	if c.CHKey == "" {
		return Result{Provider: "companies_house", Country: "GB"}, fmt.Errorf("COMPANIES_HOUSE_API_KEY not configured; get a free key at https://developer.company-information.service.gov.uk/")
	}
	u := c.chURL + "/search/companies?q=" + url.QueryEscape(name) + "&items_per_page=5"
	var raw struct {
		Items []struct {
			Name        string `json:"title"`
			Number      string `json:"company_number"`
			Status      string `json:"company_status"`
			Type        string `json:"company_type"`
			DateCreated string `json:"date_of_creation"`
			Address     string `json:"address_snippet"`
		} `json:"items"`
		Total int `json:"total_results"`
	}
	if err := c.get(ctx, u, c.CHKey, &raw); err != nil {
		return Result{Provider: "companies_house", Country: "GB"}, err
	}
	out := Result{Provider: "companies_house", Country: "GB", Total: raw.Total}
	for _, it := range raw.Items {
		out.Companies = append(out.Companies, Company{Name: it.Name, Number: it.Number, Status: it.Status, Type: it.Type, Created: it.DateCreated, Address: it.Address})
	}
	return out, nil
}

// vies validates an EU VAT number against the European Commission's VIES
// service. Free, no key, covers all EU member states plus XI.
func (c *Client) vies(ctx context.Context, cc, num string) (Result, error) {
	u := fmt.Sprintf("%s/ms/%s/vat/%s", c.viesURL, url.PathEscape(cc), url.PathEscape(num))
	var raw struct {
		IsValid   bool   `json:"isValid"`
		Name      string `json:"name"`
		Address   string `json:"address"`
		UserError string `json:"userError"`
	}
	if err := c.get(ctx, u, "", &raw); err != nil {
		return Result{Provider: "vies", Country: cc}, err
	}
	out := Result{Provider: "vies", Country: cc}
	if raw.UserError != "" && raw.UserError != "VALID" {
		out.Note = raw.UserError
	}
	addr := strings.Join(strings.Fields(raw.Address), " ")
	valid := raw.IsValid
	out.Companies = append(out.Companies, Company{
		Name:      strings.TrimSpace(raw.Name),
		Address:   addr,
		VATNumber: cc + num,
		VATValid:  &valid,
	})
	return out, nil
}

// france searches the French government's open company search API
// (recherche-entreprises.api.gouv.fr). Free, no key, textual name search.
func (c *Client) france(ctx context.Context, name string) (Result, error) {
	u := c.frURL + "/search?q=" + url.QueryEscape(name) + "&per_page=5"
	var raw struct {
		Results []struct {
			NomComplet string `json:"nom_complet"`
			Siren      string `json:"siren"`
			EtatAdmin  string `json:"etat_administratif"`
			NatureJur  string `json:"nature_juridique"`
			DateCreat  string `json:"date_creation"`
			Siege      struct {
				Adresse string `json:"adresse"`
			} `json:"siege"`
		} `json:"results"`
		Total int `json:"total_results"`
	}
	if err := c.get(ctx, u, "", &raw); err != nil {
		return Result{Provider: "recherche-entreprises", Country: "FR"}, err
	}
	out := Result{Provider: "recherche-entreprises", Country: "FR", Total: raw.Total}
	for _, it := range raw.Results {
		status := "active"
		if strings.EqualFold(it.EtatAdmin, "C") {
			status = "ceased"
		}
		out.Companies = append(out.Companies, Company{Name: it.NomComplet, Number: it.Siren, Status: status, Type: it.NatureJur, Created: it.DateCreat, Address: it.Siege.Adresse})
	}
	return out, nil
}

// romania queries ANAF's free taxpayer registry by fiscal code (CUI). No key;
// name search is not offered, so the router requires fiscal_code for RO.
func (c *Client) romania(ctx context.Context, cui string) (Result, error) {
	cui = strings.TrimSpace(strings.ToUpper(cui))
	cui = strings.TrimPrefix(cui, "RO")
	var n int
	if _, err := fmt.Sscanf(cui, "%d", &n); err != nil || n <= 0 {
		return Result{Provider: "anaf", Country: "RO"}, fmt.Errorf("invalid Romanian fiscal code %q", cui)
	}
	body, _ := json.Marshal([]map[string]any{{"cui": n, "data": time.Now().Format("2006-01-02")}})
	req, err := http.NewRequestWithContext(ctx, "POST", c.roURL, bytes.NewReader(body))
	if err != nil {
		return Result{Provider: "anaf", Country: "RO"}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{Provider: "anaf", Country: "RO"}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Result{Provider: "anaf", Country: "RO"}, fmt.Errorf("anaf status %d", resp.StatusCode)
	}
	var raw struct {
		Cod   int `json:"cod"`
		Found []struct {
			Denumire string `json:"denumire"`
			CUI      int    `json:"cui"`
			Adresa   string `json:"adresa"`
			Inactiv  bool   `json:"statusInactivi"`
			Tva      bool   `json:"scpTVA"`
		} `json:"found"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Result{Provider: "anaf", Country: "RO"}, err
	}
	out := Result{Provider: "anaf", Country: "RO", Total: len(raw.Found)}
	for _, f := range raw.Found {
		status := "active"
		if f.Inactiv {
			status = "inactive"
		}
		typ := ""
		if f.Tva {
			typ = "VAT-registered"
		}
		out.Companies = append(out.Companies, Company{Name: f.Denumire, Number: fmt.Sprint(f.CUI), Status: status, Type: typ, Address: f.Adresa})
	}
	return out, nil
}

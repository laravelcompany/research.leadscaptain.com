package leadscaptain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string, timeout int) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{Timeout: time.Duration(timeout) * time.Second}}
}

type SearchParams struct {
	Q           string `json:"q"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
	Industry    string `json:"industry"`
	Page        int    `json:"page"`
	PerPage     int    `json:"per_page"`
}
type Lead struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Company   string `json:"company_name"`
	Domain    string `json:"company_domain"`
	Title     string `json:"position_title"`
	Industry  string `json:"industry_name"`
	Country   string `json:"country_code"`
	City      string `json:"city"`
	Linkedin  string `json:"linkedin_url"`
	Summary   string `json:"summary"`
}

func (c *Client) Search(ctx context.Context, p SearchParams) ([]Lead, error) {
	fmt.Printf("{\"time\":\"%s\",\"level\":\"DEBUG\",\"msg\":\"leads search\",\"q\":\"%s\",\"country\":\"%s\",\"city\":\"%s\",\"industry\":\"%s\",\"per_page\":%d}\n", time.Now().Format(time.RFC3339), p.Q, p.CountryCode, p.City, p.Industry, p.PerPage)
	if c.baseURL == "" {
		fmt.Printf("{\"time\":\"%s\",\"level\":\"DEBUG\",\"msg\":\"using mock leads\"}\n", time.Now().Format(time.RFC3339))
		return mockLeads(p), nil
	}
	u, _ := url.Parse(c.baseURL + "/leads")
	q := u.Query()
	if p.Q != "" {
		q.Set("q", p.Q)
		q.Set("position_title", p.Q)
	}
	if p.CountryCode != "" {
		q.Set("country_code", p.CountryCode)
	}
	if p.City != "" {
		q.Set("location", p.City)
	}
	if p.Industry != "" {
		q.Set("industry_name", p.Industry)
	}
	q.Set("limit", fmt.Sprint(p.PerPage))
	if p.Page != 0 {
		q.Set("page", fmt.Sprint(p.Page))
	}
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if c.token != "" {
		req.Header.Set("X-API-Token", c.token)
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("leadscaptain %d", resp.StatusCode)
	}
	var raw struct {
		Data []struct {
			Key         string   `json:"key"`
			ID          string   `json:"id"`
			FirstName   string   `json:"first_name"`
			LastName    string   `json:"last_name"`
			Company     string   `json:"company_name"`
			Domain      string   `json:"company_domain"`
			Title       string   `json:"position_title"`
			Industry    string   `json:"industry_name"`
			Country     string   `json:"country_code"`
			City        string   `json:"city"`
			Linkedin    string   `json:"linkedin_url"`
			Email       string   `json:"email"`
			Emails      []string `json:"emails"`
			PositionLoc string   `json:"position_location"`
			Summary     string   `json:"summary"`
		} `json:"data"`
		Total      int `json:"total"`
		TotalPages int `json:"total_pages"`
		Page       int `json:"page"`
		Limit      int `json:"limit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil && err != io.EOF {
		return nil, fmt.Errorf("leadscaptain decode: %w", err)
	}
	var out []Lead
	for _, d := range raw.Data {
		email := d.Email
		if email == "" && len(d.Emails) > 0 {
			email = d.Emails[0]
		}
		city := d.City
		if city == "" {
			city = d.PositionLoc
		}
		id := d.ID
		if id == "" {
			id = d.Key
		}
		out = append(out, Lead{ID: id, FirstName: d.FirstName, LastName: d.LastName, Email: email, Company: d.Company, Domain: d.Domain, Title: d.Title, Industry: d.Industry, Country: d.Country, City: city, Linkedin: d.Linkedin, Summary: d.Summary})
	}
	if len(out) > 0 && raw.TotalPages > 1 && p.PerPage > len(out) {
		for page := 2; page <= raw.TotalPages && len(out) < p.PerPage && page <= 5; page++ {
			q.Set("page", fmt.Sprint(page))
			u.RawQuery = q.Encode()
			req2, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
			if c.token != "" {
				req2.Header.Set("X-API-Token", c.token)
				req2.Header.Set("Authorization", "Bearer "+c.token)
			}
			resp2, err := c.http.Do(req2)
			if err != nil {
				break
			}
			var raw2 struct {
				Data []struct {
					Key         string   `json:"key"`
					ID          string   `json:"id"`
					FirstName   string   `json:"first_name"`
					LastName    string   `json:"last_name"`
					Company     string   `json:"company_name"`
					Domain      string   `json:"company_domain"`
					Title       string   `json:"position_title"`
					Industry    string   `json:"industry_name"`
					Country     string   `json:"country_code"`
					City        string   `json:"city"`
					Linkedin    string   `json:"linkedin_url"`
					Email       string   `json:"email"`
					Emails      []string `json:"emails"`
					PositionLoc string   `json:"position_location"`
					Summary     string   `json:"summary"`
				} `json:"data"`
			}
			json.NewDecoder(resp2.Body).Decode(&raw2)
			resp2.Body.Close()
			if len(raw2.Data) == 0 {
				break
			}
			for _, d := range raw2.Data {
				if len(out) >= p.PerPage {
					break
				}
				email := d.Email
				if email == "" && len(d.Emails) > 0 {
					email = d.Emails[0]
				}
				city := d.City
				if city == "" {
					city = d.PositionLoc
				}
				id := d.ID
				if id == "" {
					id = d.Key
				}
				out = append(out, Lead{ID: id, FirstName: d.FirstName, LastName: d.LastName, Email: email, Company: d.Company, Domain: d.Domain, Title: d.Title, Industry: d.Industry, Country: d.Country, City: city, Linkedin: d.Linkedin, Summary: d.Summary})
			}
		}
	}
	return out, nil
}

var mockSeq atomic.Int64

func mockLeads(p SearchParams) []Lead {
	firsts := []string{"Alice", "Bob", "Carol", "David", "Emma", "Frank", "Grace", "Henry", "Iris", "Jack", "Karen", "Leo", "Maya", "Noah", "Olivia", "Paul", "Quinn", "Rosa", "Sam", "Tina"}
	lasts := []string{"Smith", "Jones", "Williams", "Brown", "Taylor", "Davies", "Wilson", "Evans", "Thomas", "Roberts", "Johnson", "Lewis", "Walker", "Hall", "Allen", "Young", "Wright", "King", "Scott", "Green"}
	companies := []string{"Acme Ltd", "Globex", "Initech", "Umbrella", "Stark Industries", "Wayne Enterprises", "Hooli", "Massive Dynamic", "Wonka Industries", "Cyberdyne"}
	domains := []string{"acme.co.uk", "globex.com", "initech.io", "umbrella.tech", "stark.co.uk", "wayne.co.uk", "hooli.com", "massive.co.uk", "wonka.co.uk", "cyberdyne.ai"}
	mockSeq.Add(1)
	var r []Lead
	title := p.Q
	if title == "" {
		title = "CTO"
	}
	seed := mockSeq.Load()
	for i := 0; i < 5; i++ {
		idx := int(seed*7 + int64(i)*13)
		r = append(r, Lead{
			ID:        fmt.Sprintf("mock-%d-%d-%d", seed, i, time.Now().UnixNano()%1000),
			FirstName: firsts[idx%len(firsts)],
			LastName:  lasts[(idx*3)%len(lasts)] + fmt.Sprintf("%d", seed*5+int64(i)),
			Email:     fmt.Sprintf("%s.%s%d@%s", firsts[idx%len(firsts)], lasts[(idx*3)%len(lasts)], seed*5+int64(i), domains[(idx*5)%len(domains)]),
			Company:   companies[(idx*7)%len(companies)],
			Domain:    domains[(idx*5)%len(domains)],
			Title:     title,
			Industry:  p.Industry,
			Country:   p.CountryCode,
			City:      p.City,
			Linkedin:  fmt.Sprintf("https://linkedin.com/in/%s-%s-%d", firsts[idx%len(firsts)], lasts[(idx*3)%len(lasts)], seed*5+int64(i)),
		})
	}
	return r
}

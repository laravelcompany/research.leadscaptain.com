package leads

import (
	"encoding/csv"
	"io"
	"strings"
)

// ImportedLead is one row parsed from an uploaded CSV file.
type ImportedLead struct {
	FirstName string
	LastName  string
	Email     string
	Company   string
	Domain    string
	Title     string
	Industry  string
	Country   string
	City      string
	Linkedin  string
}

// columnAliases maps common CSV header names (lower-cased, separators removed)
// onto ImportedLead fields.
var columnAliases = map[string]string{
	"firstname": "first_name", "givenname": "first_name",
	"lastname": "last_name", "surname": "last_name", "familyname": "last_name",
	"email": "email", "emailaddress": "email", "workemail": "email",
	"company": "company", "companyname": "company", "organization": "company", "organisation": "company",
	"domain": "domain", "companydomain": "domain", "website": "domain",
	"title": "title", "jobtitle": "title", "position": "title", "positiontitle": "title", "role": "title",
	"industry": "industry", "industryname": "industry",
	"country": "country", "countrycode": "country",
	"city": "city", "location": "city",
	"linkedin": "linkedin", "linkedinurl": "linkedin", "linkedinprofile": "linkedin",
}

func normalizeHeader(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	var b strings.Builder
	for _, r := range h {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseLeadsCSV reads a CSV with a header row and returns the parsed leads.
// Rows without an email address are skipped. Unknown columns are ignored.
func ParseLeadsCSV(r io.Reader) ([]ImportedLead, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		return nil, err
	}
	mapping := map[int]string{}
	for i, h := range header {
		if field, ok := columnAliases[normalizeHeader(h)]; ok {
			mapping[i] = field
		}
	}
	var out []ImportedLead
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		var l ImportedLead
		for i, v := range rec {
			field, ok := mapping[i]
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			switch field {
			case "first_name":
				l.FirstName = v
			case "last_name":
				l.LastName = v
			case "email":
				l.Email = v
			case "company":
				l.Company = v
			case "domain":
				l.Domain = v
			case "title":
				l.Title = v
			case "industry":
				l.Industry = v
			case "country":
				l.Country = v
			case "city":
				l.City = v
			case "linkedin":
				l.Linkedin = v
			}
		}
		if l.Email == "" {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

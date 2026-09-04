package scoring

import (
	"encoding/json"
	"strings"
)

type Breakdown map[string]int

type Result struct {
	Score     int       `json:"score"`
	Breakdown Breakdown `json:"breakdown"`
}

func Score(title, industry, location, emailStatus, companyDomain string, targetTitles, targetIndustries, targetLocations []string) Result {
	b := Breakdown{}
	score := 0
	if containsFold(targetTitles, title) {
		b["title_match"] = 30
		score += 30
	}
	if containsFold(targetIndustries, industry) {
		b["industry_match"] = 20
		score += 20
	}
	if containsFold(targetLocations, location) {
		b["location_match"] = 15
		score += 15
	}
	if emailStatus == "valid" {
		b["verified_email"] = 15
		score += 15
	} else if emailStatus == "invalid" {
		b["invalid_email"] = -30
		score -= 30
	} else if strings.Contains(emailStatus, "generic") {
		b["generic_email"] = -20
		score -= 20
	}
	if companyDomain != "" {
		b["company_domain"] = 10
		score += 10
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return Result{Score: score, Breakdown: b}
}
func ScoreWithSummary(title, industry, location, emailStatus, companyDomain, summary string, targetTitles, targetIndustries, targetLocations []string) Result {
	r := Score(title, industry, location, emailStatus, companyDomain, targetTitles, targetIndustries, targetLocations)
	if len(summary) > 200 {
		r.Breakdown["rich_summary"] = 10
		r.Score += 10
	} else if len(summary) > 80 {
		r.Breakdown["summary"] = 5
		r.Score += 5
	} else if len(summary) == 0 {
		r.Breakdown["no_summary"] = -5
		r.Score -= 5
	}
	if summary != "" {
		ls := strings.ToLower(summary)
		for _, t := range targetTitles {
			if t != "" && strings.Contains(ls, strings.ToLower(t)) {
				r.Breakdown["summary_title"] = 5
				r.Score += 5
				break
			}
		}
		for _, t := range targetIndustries {
			if t != "" && strings.Contains(ls, strings.ToLower(t)) {
				r.Breakdown["summary_industry"] = 5
				r.Score += 5
				break
			}
		}
	}
	if r.Score < 0 {
		r.Score = 0
	}
	if r.Score > 100 {
		r.Score = 100
	}
	return r
}
func (r Result) JSON() string { b, _ := json.Marshal(r.Breakdown); return string(b) }
func containsFold(list []string, v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, x := range list {
		if strings.ToLower(x) == v {
			return true
		}
		if v != "" && strings.Contains(v, strings.ToLower(x)) {
			return true
		}
	}
	return false
}

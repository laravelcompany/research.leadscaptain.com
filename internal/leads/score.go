package leads

import "strings"

type Weights struct{ ICP, DataQuality, Intent, Engagement float64 }

var DefaultWeights = Weights{0.45, 0.20, 0.20, 0.15}

func ICPScore(title, industry, location string) int {
	s := 0
	t := strings.ToLower(title)
	if strings.Contains(t, "cto") || strings.Contains(t, "chief technology") || strings.Contains(t, "vp engineering") || strings.Contains(t, "head of engineering") {
		s += 40
	}
	if strings.Contains(strings.ToLower(industry), "saas") || strings.Contains(strings.ToLower(industry), "software") {
		s += 30
	}
	if strings.Contains(strings.ToLower(location), "london") || strings.Contains(strings.ToLower(location), "gb") {
		s += 30
	}
	if s > 100 {
		s = 100
	}
	return s
}
func DataQualityScore(emailStatus, domain string, hasCompany bool) int {
	s := 0
	if emailStatus == "valid" {
		s += 50
	} else if emailStatus == "risky" {
		s += 20
	}
	if domain != "" {
		s += 25
	}
	if hasCompany {
		s += 25
	}
	if s > 100 {
		s = 100
	}
	return s
}
func Overall(icp, dq, intent, eng int, w Weights) int {
	return int(float64(icp)*w.ICP + float64(dq)*w.DataQuality + float64(intent)*w.Intent + float64(eng)*w.Engagement)
}

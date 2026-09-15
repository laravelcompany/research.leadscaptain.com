package agent

import (
	"fmt"
	"sort"
	"strings"

	"research-leads/internal/ai"
)

// BuildPrompt renders the planning prompt: the shared planner instructions,
// the live tool list from the registry, the objective and its progress, and
// the queries already tried. Pure so tests can pin the contract.
func BuildPrompt(desc string, target *int, total, qualified, minScore int, toolDescs, recentQueries []string) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(ai.PlannerPrompt))
	b.WriteString("\n\nObjective: ")
	b.WriteString(desc)
	b.WriteString("\nProgress: ")
	b.WriteString(fmt.Sprintf("%d leads stored, %d at or above minimum score %d", total, qualified, minScore))
	if target != nil {
		b.WriteString(fmt.Sprintf("; target is %d", *target))
	}
	b.WriteString("\nAvailable tools:\n")
	sorted := append([]string(nil), toolDescs...)
	sort.Strings(sorted)
	for _, d := range sorted {
		b.WriteString("- ")
		b.WriteString(d)
		b.WriteString("\n")
	}
	b.WriteString("Control actions (no parameters): reflect, complete_objective, wait\n")
	if len(recentQueries) > 0 {
		b.WriteString("Searches already run (do not repeat): ")
		b.WriteString(strings.Join(recentQueries, " | "))
		b.WriteString("\n")
	}
	b.WriteString(`Respond with ONLY one JSON object: {"action":"<tool or control action>","reason":"...","parameters":{...}}`)
	return b.String()
}

// DeriveSearchParams extracts a sensible search query from a free-text
// objective for the no-AI fallback: a quoted phrase if present, otherwise a
// recognised title keyword, otherwise the leading words; plus a country code
// and city when the description names one.
func DeriveSearchParams(desc string) (q, countryCode, city, industry string) {
	lower := strings.ToLower(desc)
	// Quoted phrases are the strongest signal of intent.
	if i := strings.Index(desc, `"`); i >= 0 {
		if j := strings.Index(desc[i+1:], `"`); j > 0 {
			q = desc[i+1 : i+1+j]
		}
	}
	titles := []string{"cto", "ceo", "cfo", "coo", "founder", "co-founder", "owner", "managing director", "director", "vp engineering", "head of engineering", "head of marketing", "marketing manager", "sales manager", "developer", "engineer", "architect", "dentist", "plumber", "accountant", "solicitor", "lawyer", "agency owner", "consultant", "recruiter"}
	if q == "" {
		for _, t := range titles {
			if strings.Contains(lower, t) {
				q = t
				break
			}
		}
	}
	if q == "" {
		words := strings.Fields(desc)
		if len(words) > 4 {
			words = words[:4]
		}
		q = strings.Join(words, " ")
	}
	countries := map[string]string{
		"united kingdom": "GB", " uk ": "GB", "britain": "GB", "england": "GB", "scotland": "GB", "wales": "GB",
		"romania": "RO", "germany": "DE", "france": "FR", "spain": "ES", "italy": "IT", "netherlands": "NL",
		"poland": "PL", "portugal": "PT", "ireland": "IE", "sweden": "SE", "norway": "NO", "denmark": "DK",
		"united states": "US", " usa ": "US", "canada": "CA", "australia": "AU", "india": "IN",
	}
	padded := " " + lower + " "
	for name, code := range countries {
		if strings.Contains(padded, name) {
			countryCode = code
			break
		}
	}
	cities := []string{"London", "Manchester", "Birmingham", "Leeds", "Bristol", "Bucharest", "Cluj", "Iasi", "Timisoara", "Vaslui", "Berlin", "Munich", "Hamburg", "Paris", "Lyon", "Madrid", "Barcelona", "Lisbon", "Porto", "Amsterdam", "Rotterdam", "Dublin", "Warsaw", "Krakow", "Stockholm", "Copenhagen", "Vienna", "Brussels", "New York", "San Francisco", "Toronto", "Sydney"}
	lowerCity := map[string]string{}
	for _, c := range cities {
		lowerCity[strings.ToLower(c)] = c
	}
	for lc, c := range lowerCity {
		if strings.Contains(lower, lc) {
			city = c
			break
		}
	}
	industries := []string{"saas", "software", "fintech", "ecommerce", "e-commerce", "healthcare", "real estate", "construction", "legal", "accounting", "marketing", "recruitment", "hospitality", "manufacturing", "logistics", "education"}
	for _, ind := range industries {
		if strings.Contains(lower, ind) {
			industry = ind
			break
		}
	}
	return q, countryCode, city, industry
}

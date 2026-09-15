package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// CheckDomainTool inspects a domain's DNS email posture: MX records, SPF and
// DMARC. Genuinely free (plain DNS lookups) and useful both to sanity-check a
// lead's email domain and to fingerprint the company's mail provider.
type CheckDomainTool struct {
	lookupMX  func(string) ([]*net.MX, error)
	lookupTXT func(string) ([]string, error)
}

func NewCheckDomain() *CheckDomainTool {
	return &CheckDomainTool{lookupMX: net.LookupMX, lookupTXT: net.LookupTXT}
}
func (t *CheckDomainTool) Name() string { return "check_domain" }
func (t *CheckDomainTool) Description() string {
	return "DNS email posture for a domain: {\"domain\":\"acme.com\"}. Returns mx records, mail provider guess, has_spf, has_dmarc. Free, no key."
}
func (t *CheckDomainTool) Execute(ctx context.Context, input json.RawMessage) (any, error) {
	var p struct {
		Domain string `json:"domain"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return nil, err
	}
	d := strings.TrimSpace(strings.ToLower(p.Domain))
	if d == "" || strings.ContainsAny(d, "/?# @") {
		return nil, fmt.Errorf("check_domain needs a bare domain like acme.com")
	}
	res := map[string]any{"domain": d}
	mx, mxErr := t.lookupMX(d)
	var hosts []string
	for _, m := range mx {
		hosts = append(hosts, strings.TrimSuffix(m.Host, "."))
	}
	res["mx"] = hosts
	res["has_mx"] = mxErr == nil && len(hosts) > 0
	res["mx_provider"] = mailProvider(hosts)
	txts, _ := t.lookupTXT(d)
	spf := false
	for _, tx := range txts {
		if strings.HasPrefix(strings.ToLower(tx), "v=spf1") {
			spf = true
		}
	}
	res["has_spf"] = spf
	dmarcTxts, _ := t.lookupTXT("_dmarc." + d)
	dmarc := false
	for _, tx := range dmarcTxts {
		if strings.HasPrefix(strings.ToLower(tx), "v=dmarc1") {
			dmarc = true
		}
	}
	res["has_dmarc"] = dmarc
	return res, nil
}

// mailProvider guesses the mail host from MX hostnames.
func mailProvider(mx []string) string {
	joined := strings.ToLower(strings.Join(mx, " "))
	switch {
	case joined == "":
		return ""
	case strings.Contains(joined, "google"):
		return "google-workspace"
	case strings.Contains(joined, "outlook") || strings.Contains(joined, "microsoft"):
		return "microsoft-365"
	case strings.Contains(joined, "proton"):
		return "proton"
	case strings.Contains(joined, "mimecast"):
		return "mimecast"
	case strings.Contains(joined, "zoho"):
		return "zoho"
	default:
		return "other"
	}
}

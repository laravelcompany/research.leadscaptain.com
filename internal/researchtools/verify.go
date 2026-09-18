// Package researchtools backs the Tools page: keyless email verification
// (syntax + MX + provider detection), RDAP domain age checks, LinkedIn URL
// formatting, and queued bulk processing. Nothing here costs money.
package researchtools

import (
	"context"
	"net"
	"net/mail"
	"strings"
)

// EmailVerdict is the result of verifying one address.
type EmailVerdict struct {
	Email    string   `json:"email"`
	Status   string   `json:"status"` // valid | invalid | risky | unknown
	Provider string   `json:"provider,omitempty"`
	MXFound  bool     `json:"mx_found"`
	MXHosts  []string `json:"mx_hosts,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	// SMTP holds the deliverability probe verdict (one of the SMTP* constants)
	// when SMTP probing ran; empty when probing is disabled or not applicable.
	SMTP string `json:"smtp,omitempty"`
}

// disposableDomains are throwaway providers; mail to them is deliverable but
// worthless for outreach, so they score "risky".
var disposableDomains = map[string]bool{
	"mailinator.com": true, "tempmail.com": true, "temp-mail.org": true,
	"guerrillamail.com": true, "10minutemail.com": true, "yopmail.com": true,
	"throwawaymail.com": true, "fakeinbox.com": true, "sharklasers.com": true,
	"getnada.com": true, "dispostable.com": true, "maildrop.cc": true,
}

// rolePrefixes are function addresses, not people; risky for outreach.
var rolePrefixes = map[string]bool{
	"info": true, "admin": true, "sales": true, "support": true, "contact": true,
	"hello": true, "office": true, "mail": true, "noreply": true, "no-reply": true,
	"postmaster": true, "webmaster": true, "abuse": true,
}

// mxProviders maps MX host fragments to a friendly provider name.
var mxProviders = []struct {
	Fragment string
	Name     string
}{
	{"google.com", "Gmail / Google Workspace"},
	{"googlemail.com", "Gmail / Google Workspace"},
	{"outlook.com", "Outlook / Microsoft 365"},
	{"microsoft.com", "Outlook / Microsoft 365"},
	{"yahoodns.net", "Yahoo"},
	{"protonmail.ch", "ProtonMail"},
	{"proton.me", "ProtonMail"},
	{"zoho.com", "Zoho Mail"},
	{"icloud.com", "iCloud Mail"},
	{"mimecast.com", "Mimecast (corporate gateway)"},
	{"pphosted.com", "Proofpoint (corporate gateway)"},
	{"barracudanetworks.com", "Barracuda (corporate gateway)"},
}

// MXLookup resolves MX records; a field so tests can stub DNS.
type MXLookup func(ctx context.Context, domain string) ([]*net.MX, error)

// VerifyEmail checks an address without any external paid service: syntax,
// disposable/role heuristics, then MX records and provider identification.
func VerifyEmail(ctx context.Context, email string, lookup MXLookup) EmailVerdict {
	email = strings.TrimSpace(strings.ToLower(email))
	v := EmailVerdict{Email: email}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email, "@") {
		v.Status = "invalid"
		v.Reason = "malformed address"
		return v
	}
	parts := strings.SplitN(email, "@", 2)
	local, domain := parts[0], parts[1]
	if !strings.Contains(domain, ".") {
		v.Status = "invalid"
		v.Reason = "domain has no TLD"
		return v
	}
	if disposableDomains[domain] {
		v.Status = "risky"
		v.Provider = "disposable provider"
		v.Reason = "disposable email domain"
		return v
	}
	if lookup == nil {
		lookup = func(ctx context.Context, d string) ([]*net.MX, error) {
			return net.DefaultResolver.LookupMX(ctx, d)
		}
	}
	mxs, err := lookup(ctx, domain)
	if err != nil || len(mxs) == 0 {
		// No MX: some domains receive on their A record, so only call it
		// invalid when the domain does not resolve at all.
		if _, aerr := net.DefaultResolver.LookupHost(ctx, domain); aerr != nil {
			v.Status = "invalid"
			v.Reason = "domain does not resolve"
			return v
		}
		v.Status = "unknown"
		v.Reason = "no MX record (domain resolves; may still accept mail)"
		return v
	}
	v.MXFound = true
	for _, mx := range mxs {
		host := strings.TrimSuffix(mx.Host, ".")
		v.MXHosts = append(v.MXHosts, host)
		if v.Provider == "" {
			for _, p := range mxProviders {
				if strings.Contains(host, p.Fragment) {
					v.Provider = p.Name
					break
				}
			}
		}
	}
	if v.Provider == "" {
		v.Provider = "custom / other (" + v.MXHosts[0] + ")"
	}
	v.Status = "valid"
	v.Reason = "domain accepts mail"
	if rolePrefixes[local] {
		v.Status = "risky"
		v.Reason = "role address, not a person"
	}
	return v
}

// Verifier is the shared email checker: VerifyEmail's keyless syntax/MX
// stage, optionally followed by an SMTP deliverability probe when a Prober is
// attached. A nil Prober keeps the DNS-only behavior.
type Verifier struct {
	Lookup MXLookup
	Prober *SMTPProber
}

// Verify runs syntax, disposable/role and MX checks, then - when the domain
// accepts mail and a Prober is configured - an SMTP probe of the mailbox
// itself, including catch-all detection. SMTP results refine the verdict:
// deliverable confirms valid, undeliverable marks invalid, catch-all marks
// risky (the address cannot be distinguished from a dead one).
func (v *Verifier) Verify(ctx context.Context, email string) EmailVerdict {
	var lookup MXLookup
	if v != nil {
		lookup = v.Lookup
	}
	verdict := VerifyEmail(ctx, email, lookup)
	if v == nil || v.Prober == nil || !verdict.MXFound || verdict.Status != "valid" {
		return verdict
	}
	st, reason := v.Prober.Probe(ctx, verdict.Email, verdict.MXHosts)
	verdict.SMTP = st
	switch st {
	case SMTPDeliverable:
		verdict.Reason = reason
	case SMTPUndeliverable:
		verdict.Status = "invalid"
		verdict.Reason = reason
	case SMTPCatchAll:
		verdict.Status = "risky"
		verdict.Reason = reason
	case SMTPUnknown:
		verdict.Reason += " (SMTP inconclusive: " + reason + ")"
	case SMTPSkipped:
		verdict.Reason += " (SMTP not probed: port 25 unreachable from this host)"
	}
	return verdict
}

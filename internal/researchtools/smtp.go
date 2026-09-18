package researchtools

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"sync"
	"time"
)

// SMTP verdicts returned by Prober.Probe.
const (
	SMTPDeliverable   = "deliverable"
	SMTPUndeliverable = "undeliverable"
	SMTPCatchAll      = "catch_all"
	SMTPUnknown       = "unknown"
	SMTPSkipped       = "skipped"
)

// smtpBreakerTripsAt is how many consecutive transport failures (dial or
// greeting) disable probing for the life of the prober. Container hosts and
// cloud VPCs commonly block outbound port 25; the breaker stops every
// verification from eating a full timeout once that is detected.
const smtpBreakerTripsAt = 3

// DialFunc opens a connection to an SMTP server; a field so tests can inject
// an in-process fake server.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// SMTPProber verifies a mailbox by speaking SMTP to the domain's MX hosts:
// EHLO, MAIL FROM, RCPT TO. A random local part is probed after an accepted
// recipient to detect catch-all domains. It never sends a message.
type SMTPProber struct {
	Helo    string        // EHLO/HELO name; defaults to localhost.localdomain
	From    string        // MAIL FROM address; defaults to probe@localhost
	Timeout time.Duration // per-connection deadline; defaults to 8s
	Dial    DialFunc      // nil = direct TCP on port 25
	// MaxMX caps how many MX hosts are tried per address (default 2, so a
	// dead primary does not stall the pipeline).
	MaxMX int

	mu       sync.Mutex
	failures int
	broken   bool
	catchAll map[string]bool
}

func (p *SMTPProber) helo() string {
	if p.Helo != "" {
		return p.Helo
	}
	return "localhost.localdomain"
}

func (p *SMTPProber) from() string {
	if p.From != "" {
		return p.From
	}
	return "probe@localhost"
}

func (p *SMTPProber) timeout() time.Duration {
	if p.Timeout > 0 {
		return p.Timeout
	}
	return 8 * time.Second
}

// Probe checks whether mxHosts accept email. It returns one of the SMTP*
// verdicts plus a human-readable reason. Transport failures across all MX
// hosts yield SMTPUnknown; once the circuit breaker trips (the host cannot
// reach port 25 at all) every later call short-circuits to SMTPSkipped.
func (p *SMTPProber) Probe(ctx context.Context, email string, mxHosts []string) (string, string) {
	p.mu.Lock()
	if p.broken {
		p.mu.Unlock()
		return SMTPSkipped, "smtp probing disabled: outbound port 25 unreachable from this host"
	}
	p.mu.Unlock()
	if len(mxHosts) == 0 {
		return SMTPUnknown, "no MX hosts to probe"
	}
	max := p.MaxMX
	if max <= 0 {
		max = 2
	}
	if len(mxHosts) > max {
		mxHosts = mxHosts[:max]
	}
	lastReason := "no MX host answered"
	for _, host := range mxHosts {
		st, reason, transportErr := p.probeHost(ctx, host, email)
		if transportErr {
			lastReason = reason
			if p.recordFailure() {
				return SMTPSkipped, "smtp probing disabled: outbound port 25 unreachable from this host"
			}
			continue
		}
		p.recordSuccess()
		if st == SMTPUnknown {
			// Greylisting or a policy rejection: not a transport problem, but
			// another MX may answer definitively.
			lastReason = reason
			continue
		}
		return st, reason
	}
	return SMTPUnknown, lastReason
}

func (p *SMTPProber) recordFailure() (tripped bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures++
	if p.failures >= smtpBreakerTripsAt {
		p.broken = true
	}
	return p.broken
}

func (p *SMTPProber) recordSuccess() {
	p.mu.Lock()
	p.failures = 0
	p.mu.Unlock()
}

func (p *SMTPProber) isCatchAll(domain string) (known bool, is bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.catchAll == nil {
		return false, false
	}
	is, known = p.catchAll[domain]
	return known, is
}

func (p *SMTPProber) setCatchAll(domain string, is bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.catchAll == nil {
		p.catchAll = map[string]bool{}
	}
	p.catchAll[domain] = is
}

// probeHost runs one SMTP session against host. transportErr reports that the
// failure was at the connection level (dial or greeting), which feeds the
// circuit breaker; protocol-level answers never trip it.
func (p *SMTPProber) probeHost(ctx context.Context, host, email string) (status, reason string, transportErr bool) {
	dial := p.Dial
	if dial == nil {
		d := &net.Dialer{Timeout: p.timeout()}
		dial = d.DialContext
	}
	conn, err := dial(ctx, "tcp", net.JoinHostPort(host, "25"))
	if err != nil {
		return "", fmt.Sprintf("connect to %s: %v", host, err), true
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(p.timeout()))

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return "", fmt.Sprintf("greeting from %s: %v", host, err), true
	}
	defer client.Close()
	if err := client.Hello(p.helo()); err != nil {
		// A refused EHLO says the server does not want probes, not that the
		// network is blocked - inconclusive, but not a breaker failure.
		return SMTPUnknown, fmt.Sprintf("EHLO refused by %s", host), false
	}
	if err := client.Mail(p.from()); err != nil {
		return SMTPUnknown, fmt.Sprintf("MAIL FROM refused by %s", host), false
	}

	err = client.Rcpt(email)
	if tpErr, ok := err.(*textproto.Error); ok {
		switch {
		case tpErr.Code >= 500 && tpErr.Code < 600:
			return SMTPUndeliverable, fmt.Sprintf("mailbox rejected by %s (%d %s)", host, tpErr.Code, tpErr.Msg), false
		case tpErr.Code >= 400:
			return SMTPUnknown, fmt.Sprintf("temporary rejection by %s (%d, likely greylisting)", host, tpErr.Code), false
		}
	}
	if err != nil {
		return SMTPUnknown, fmt.Sprintf("RCPT failed at %s: %v", host, err), false
	}

	// Recipient accepted. Catch-all domains accept every local part, so probe
	// a random one; an accepted random address downgrades to catch-all.
	domain := email[strings.LastIndex(email, "@")+1:]
	if known, is := p.isCatchAll(domain); known {
		if is {
			return SMTPCatchAll, fmt.Sprintf("catch-all domain (cached) via %s", host), false
		}
		return SMTPDeliverable, fmt.Sprintf("mailbox accepted by %s", host), false
	}
	rnd := make([]byte, 8)
	rand.Read(rnd)
	randomAddr := "probe-" + hex.EncodeToString(rnd) + "@" + domain
	randErr := client.Rcpt(randomAddr)
	if randErr == nil {
		p.setCatchAll(domain, true)
		return SMTPCatchAll, fmt.Sprintf("catch-all domain: %s accepts any address", host), false
	}
	if tpErr, ok := randErr.(*textproto.Error); ok && tpErr.Code >= 500 {
		p.setCatchAll(domain, false)
		return SMTPDeliverable, fmt.Sprintf("mailbox accepted by %s", host), false
	}
	// Inconclusive random probe (timeout, 4xx): the real address was accepted,
	// so deliverable stands but catch-all stays unknown.
	return SMTPDeliverable, fmt.Sprintf("mailbox accepted by %s (catch-all undetermined)", host), false
}

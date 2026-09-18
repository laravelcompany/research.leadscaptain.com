package researchtools

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeSMTPServer speaks just enough SMTP for the prober: greeting, EHLO,
// MAIL, and RCPT answers driven by the script map (local part -> code).
type fakeSMTPServer struct {
	ln     net.Listener
	script map[string]int
}

func newFakeSMTP(t *testing.T, script map[string]int) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTPServer{ln: ln, script: script}
	go f.serve()
	t.Cleanup(func() { ln.Close() })
	return f
}

func (f *fakeSMTPServer) serve() {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			r := bufio.NewReader(c)
			fmt.Fprintf(c, "220 fake.test ESMTP ready\r\n")
			for {
				line, err := r.ReadString('\n')
				if err != nil {
					return
				}
				up := strings.ToUpper(strings.TrimSpace(line))
				switch {
				case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
					fmt.Fprintf(c, "250-fake.test\r\n250 8BITMIME\r\n")
				case strings.HasPrefix(up, "MAIL FROM"):
					fmt.Fprintf(c, "250 OK\r\n")
				case strings.HasPrefix(up, "RCPT TO"):
					local := "?"
					if i := strings.Index(line, "<"); i >= 0 {
						if j := strings.Index(line[i:], "@"); j >= 0 {
							local = line[i+1 : i+j]
						}
					}
					code, ok := f.script[local]
					if !ok {
						code, ok = f.script["*"]
					}
					if !ok {
						code = 550
					}
					if code >= 500 {
						fmt.Fprintf(c, "%d no such user\r\n", code)
					} else if code >= 400 {
						fmt.Fprintf(c, "%d try later\r\n", code)
					} else {
						fmt.Fprintf(c, "250 OK\r\n")
					}
				case strings.HasPrefix(up, "QUIT"):
					fmt.Fprintf(c, "221 bye\r\n")
					return
				default:
					fmt.Fprintf(c, "250 OK\r\n")
				}
			}
		}(conn)
	}
}

// dialTo rewrites any address to the fake server's listener.
func (f *fakeSMTPServer) dialTo(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 2 * time.Second}
	return d.DialContext(ctx, network, f.ln.Addr().String())
}

func TestProbeDeliverableAndCatchAll(t *testing.T) {
	// Catch-all: the random probe address is not in the script's spirit - the
	// server answers 250 for "cto" and the probe local part below.
	f := newFakeSMTP(t, map[string]int{"cto": 250})
	p := &SMTPProber{Dial: f.dialTo, Timeout: 3 * time.Second}
	st, reason := p.Probe(context.Background(), "cto@acme.com", []string{"mx.acme.com"})
	// Random part gets 550 from the script default, so this is deliverable.
	if st != SMTPDeliverable {
		t.Fatalf("expected deliverable, got %q (%s)", st, reason)
	}

	// Catch-all server: every RCPT (real and random probe) answered 250.
	f2 := newFakeSMTP(t, map[string]int{"*": 250})
	p2 := &SMTPProber{Dial: f2.dialTo, Timeout: 3 * time.Second}
	st2, _ := p2.Probe(context.Background(), "cto@acme.com", []string{"mx.acme.com"})
	if st2 != SMTPCatchAll {
		t.Fatalf("expected catch_all, got %q", st2)
	}
	// Cached: a second probe for the domain must not need another random hit.
	if known, is := p2.isCatchAll("acme.com"); !known || !is {
		t.Fatal("catch-all verdict must be cached per domain")
	}
}

func TestProbeUndeliverable(t *testing.T) {
	f := newFakeSMTP(t, map[string]int{"gone": 550})
	p := &SMTPProber{Dial: f.dialTo, Timeout: 3 * time.Second}
	st, _ := p.Probe(context.Background(), "gone@acme.com", []string{"mx.acme.com"})
	if st != SMTPUndeliverable {
		t.Fatalf("expected undeliverable, got %q", st)
	}
}

func TestProbeGreylistedIsUnknown(t *testing.T) {
	f := newFakeSMTP(t, map[string]int{"cto": 450})
	p := &SMTPProber{Dial: f.dialTo, Timeout: 3 * time.Second}
	st, _ := p.Probe(context.Background(), "cto@acme.com", []string{"mx.acme.com"})
	if st != SMTPUnknown {
		t.Fatalf("greylisting must be unknown, got %q", st)
	}
}

func TestProbeCircuitBreakerTrips(t *testing.T) {
	dialErr := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, fmt.Errorf("connect: network is unreachable")
	}
	p := &SMTPProber{Dial: dialErr, Timeout: time.Second, MaxMX: 1}
	// 1 MX host per probe -> one transport failure each; breaker trips at 3.
	if st, _ := p.Probe(context.Background(), "a@x.com", []string{"mx1"}); st != SMTPUnknown {
		t.Fatalf("expected unknown, got %q", st)
	}
	p.Probe(context.Background(), "b@x.com", []string{"mx1"})
	st, reason := p.Probe(context.Background(), "c@x.com", []string{"mx1"})
	if st != SMTPSkipped {
		t.Fatalf("breaker must trip after %d failures, got %q", smtpBreakerTripsAt, st)
	}
	if !strings.Contains(reason, "port 25") {
		t.Fatalf("reason must explain the port-25 block, got %q", reason)
	}
}

func TestVerifierCombinesMXAndSMTP(t *testing.T) {
	f := newFakeSMTP(t, map[string]int{"cto": 250})
	v := &Verifier{
		Lookup: func(ctx context.Context, domain string) ([]*net.MX, error) {
			return []*net.MX{{Host: "mx.acme.com.", Pref: 10}}, nil
		},
		Prober: &SMTPProber{Dial: f.dialTo, Timeout: 3 * time.Second},
	}
	got := v.Verify(context.Background(), "cto@acme.com")
	if got.Status != "valid" || got.SMTP != SMTPDeliverable {
		t.Fatalf("expected valid+deliverable, got %+v", got)
	}
	// Without a prober the MX stage alone decides (previous behavior).
	v2 := &Verifier{Lookup: v.Lookup}
	got2 := v2.Verify(context.Background(), "cto@acme.com")
	if got2.Status != "valid" || got2.SMTP != "" {
		t.Fatalf("DNS-only verifier must not probe, got %+v", got2)
	}
	// Undeliverable mailbox flips the verdict to invalid.
	f3 := newFakeSMTP(t, map[string]int{"ghost": 550})
	v3 := &Verifier{Lookup: v.Lookup, Prober: &SMTPProber{Dial: f3.dialTo, Timeout: 3 * time.Second}}
	if got3 := v3.Verify(context.Background(), "ghost@acme.com"); got3.Status != "invalid" {
		t.Fatalf("expected invalid, got %+v", got3)
	}
}

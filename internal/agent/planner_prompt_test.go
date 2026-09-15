package agent

import (
	"strings"
	"testing"
)

func TestBuildPromptIncludesToolsProgressAndHistory(t *testing.T) {
	target := 25
	p := BuildPrompt("Find UK CTOs", &target, 10, 4, 70,
		[]string{"search_leads: Search LeadsCaptain", "verify_email: Verify email"},
		[]string{"CTO GB London"})
	for _, want := range []string{"Find UK CTOs", "target is 25", "10 leads stored", "search_leads: Search LeadsCaptain", "verify_email: Verify email", "do not repeat", "CTO GB London", "complete_objective", `"action"`} {
		if !strings.Contains(p, want) {
			t.Fatalf("prompt missing %q:\n%s", want, p)
		}
	}
}

func TestBuildPromptWithoutTarget(t *testing.T) {
	p := BuildPrompt("Explore", nil, 0, 0, 0, nil, nil)
	if strings.Contains(p, "; target is") {
		t.Fatalf("no-target objective must not mention a target:\n%s", p)
	}
	if strings.Contains(p, "do not repeat") {
		t.Fatalf("empty history must not add the repeat warning:\n%s", p)
	}
}

func TestDeriveSearchParamsQuotedPhraseWins(t *testing.T) {
	q, cc, city, ind := DeriveSearchParams(`Find "growth marketers" in London, UK SaaS companies`)
	if q != "growth marketers" {
		t.Fatalf("quoted phrase should win, got %q", q)
	}
	if cc != "GB" || city != "London" || ind != "saas" {
		t.Fatalf("bad params: %q %q %q", cc, city, ind)
	}
}

func TestDeriveSearchParamsTitleAndCountry(t *testing.T) {
	q, cc, city, _ := DeriveSearchParams("Research dental clinic owners in Romania")
	if q != "owner" {
		t.Fatalf("expected title keyword, got %q", q)
	}
	if cc != "RO" {
		t.Fatalf("expected RO, got %q", cc)
	}
	_ = city
}

func TestDeriveSearchParamsFallsBackToLeadingWords(t *testing.T) {
	q, _, _, _ := DeriveSearchParams("xyzzy foobarbaz quux corge grault")
	if q != "xyzzy foobarbaz quux corge" {
		t.Fatalf("expected first four words, got %q", q)
	}
	if q == "CTO" {
		t.Fatal("must not hardcode CTO")
	}
}

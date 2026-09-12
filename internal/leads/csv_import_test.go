package leads

import (
	"strings"
	"testing"
)

func TestParseLeadsCSV(t *testing.T) {
	data := "First Name,Last Name,Work Email,Company,Job Title,Country\n" +
		"Ada,Lovelace,ada@example.com,Analytical Engines,CTO,GB\n" +
		"NoEmail,Person,,NoMail Inc,Intern,US\n"
	leads, err := ParseLeadsCSV(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(leads) != 1 {
		t.Fatalf("expected 1 lead (rows without email skipped), got %d", len(leads))
	}
	l := leads[0]
	if l.FirstName != "Ada" || l.Email != "ada@example.com" || l.Title != "CTO" || l.Country != "GB" {
		t.Fatalf("unexpected lead: %+v", l)
	}
}

func TestParseLeadsCSVEmpty(t *testing.T) {
	if _, err := ParseLeadsCSV(strings.NewReader("")); err == nil {
		t.Fatal("expected error for empty input")
	}
}

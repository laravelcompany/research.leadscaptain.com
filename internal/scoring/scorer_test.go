package scoring

import "testing"

func TestScoreTitleIndustryLocation(t *testing.T) {
	r := Score("CTO", "Software", "GB", "valid", "acme.com", []string{"CTO"}, []string{"Software"}, []string{"GB"})
	if r.Score != 30+20+15+15+10 {
		t.Fatalf("expected 90, got %d (%v)", r.Score, r.Breakdown)
	}
}

func TestScoreClampsAtZero(t *testing.T) {
	r := Score("Intern", "Retail", "FR", "invalid", "", []string{"CTO"}, []string{"Software"}, []string{"GB"})
	if r.Score != 0 {
		t.Fatalf("expected clamped 0, got %d", r.Score)
	}
}

func TestScoreWithSummaryBonus(t *testing.T) {
	long := ""
	for i := 0; i < 250; i++ {
		long += "x"
	}
	r := ScoreWithSummary("CTO", "Software", "GB", "unchecked", "", long, []string{"CTO"}, []string{"Software"}, []string{"GB"})
	if r.Breakdown["rich_summary"] != 10 {
		t.Fatalf("expected rich_summary bonus, got %v", r.Breakdown)
	}
	if r.Score > 100 {
		t.Fatalf("score must cap at 100, got %d", r.Score)
	}
}

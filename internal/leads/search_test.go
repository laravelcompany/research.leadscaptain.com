package leads

import "testing"

func TestBuildWhereKnownField(t *testing.T) {
	where, args, err := BuildWhere([]Filter{{Field: "title", Operator: "CONTAINS", Value: "CTO"}})
	if err != nil {
		t.Fatal(err)
	}
	if where != "position_title LIKE ?" {
		t.Fatalf("unexpected where: %s", where)
	}
	if len(args) != 1 || args[0] != "%CTO%" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestBuildWhereRejectsInjection(t *testing.T) {
	_, _, err := BuildWhere([]Filter{{Field: "1=1; DROP TABLE leads; --", Operator: "=", Value: "x"}})
	if err == nil {
		t.Fatal("expected injection attempt to be rejected")
	}
}

func TestBuildWhereUnknownOperator(t *testing.T) {
	_, _, err := BuildWhere([]Filter{{Field: "email", Operator: "REGEXP", Value: "x"}})
	if err == nil {
		t.Fatal("expected unsupported operator to be rejected")
	}
}

func TestCanTransition(t *testing.T) {
	if !CanTransition("NEW", "DISCOVERED") {
		t.Fatal("NEW -> DISCOVERED should be allowed")
	}
	if CanTransition("NEW", "CONVERTED") {
		t.Fatal("NEW -> CONVERTED should be rejected")
	}
	if !CanTransition("CONVERTED", "ARCHIVED") {
		t.Fatal("ARCHIVED should be reachable from anywhere")
	}
}

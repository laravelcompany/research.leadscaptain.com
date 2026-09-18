package researchtools

import (
	"database/sql"
	"testing"

	"research-leads/internal/db"
)

func testDBTools(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

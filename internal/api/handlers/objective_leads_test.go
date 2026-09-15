package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"research-leads/internal/db"
	"research-leads/internal/events"
)

func objectiveTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
func withID(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
func TestObjectiveByIDReportsOnlyItsLeadProgress(t *testing.T) {
	d := objectiveTestDB(t)
	d.Exec("INSERT INTO objectives(id,name,description,status,target_leads,minimum_score) VALUES(1,'One','first','running',2,70),(2,'Two','second','idle',5,70)")
	d.Exec("INSERT INTO leads(first_name,lead_score,objective_id) VALUES('A',80,1),('B',20,1),('C',99,2)")
	s := &Server{DB: d}
	rec := httptest.NewRecorder()
	s.ObjectiveByID(rec, withID(httptest.NewRequest("GET", "/api/v1/objectives/1", nil), "1"))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	var got map[string]any
	json.NewDecoder(rec.Body).Decode(&got)
	if got["leads_discovered"] != float64(2) || got["qualified"] != float64(1) {
		t.Fatalf("wrong scoped counts: %#v", got)
	}
}
func TestObjectiveByIDInvalidAndMissingAreNotFound(t *testing.T) {
	d := objectiveTestDB(t)
	s := &Server{DB: d}
	for _, id := range []string{"abc", "999"} {
		rec := httptest.NewRecorder()
		s.ObjectiveByID(rec, withID(httptest.NewRequest("GET", "/api/v1/objectives/"+id, nil), id))
		if rec.Code != 404 {
			t.Fatalf("%s: got %d", id, rec.Code)
		}
	}
}
func TestLeadsFiltersAndPaginatesByObjective(t *testing.T) {
	d := objectiveTestDB(t)
	d.Exec("INSERT INTO objectives(id,name,description,status) VALUES(1,'One','','idle'),(2,'Two','','idle')")
	for i := 0; i < 30; i++ {
		oid := 1
		if i == 29 {
			oid = 2
		}
		d.Exec("INSERT INTO leads(first_name,company_name,lead_score,source,status,objective_id) VALUES(?,?,?,?,?,?)", "Lead "+strconv.Itoa(i), "Acme", 80, "test", "NEW", oid)
	}
	s := &Server{DB: d}
	rec := httptest.NewRecorder()
	s.Leads(rec, httptest.NewRequest("GET", "/api/v1/leads?objective_id=1&page=2&per_page=25&min_score=70&source=test&status=NEW", nil))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
		Last  int              `json:"last_page"`
	}
	json.NewDecoder(rec.Body).Decode(&got)
	if got.Total != 29 || got.Last != 2 || len(got.Data) != 4 {
		t.Fatalf("bad pagination: %+v", got)
	}
	for _, l := range got.Data {
		if l["objective_id"] != float64(1) {
			t.Fatalf("lead leaked: %#v", l)
		}
	}
}
func TestEventObjectiveID(t *testing.T) {
	if got := eventObjectiveID(events.Event{Payload: map[string]any{"objective_id": int64(42)}}); got != "42" {
		t.Fatal(got)
	}
	if got := eventObjectiveID(events.Event{Payload: map[string]any{"objective_id": int64(7)}}); got == "42" {
		t.Fatal("cross-objective event matched")
	}
}

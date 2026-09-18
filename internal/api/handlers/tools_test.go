package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"research-leads/internal/researchtools"
)

func toolsTestServer(t *testing.T) *Server {
	d := objectiveTestDB(t)
	return &Server{DB: d, Tools: &researchtools.Service{
		DB: d,
		Verify: func(ctx context.Context, email string) researchtools.EmailVerdict {
			return researchtools.EmailVerdict{Email: email, Status: "valid", MXFound: true, Provider: "Gmail / Google Workspace"}
		},
	}}
}

func TestToolVerifyEmailHandler(t *testing.T) {
	s := toolsTestServer(t)
	rec := httptest.NewRecorder()
	s.ToolVerifyEmail(rec, httptest.NewRequest("POST", "/api/v1/tools/verify-email", strings.NewReader(`{"email":"a@gmail.com"}`)))
	if rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	var v researchtools.EmailVerdict
	json.NewDecoder(rec.Body).Decode(&v)
	if v.Status != "valid" {
		t.Fatalf("%+v", v)
	}
}

func TestToolVerifyEmailValidation(t *testing.T) {
	s := toolsTestServer(t)
	rec := httptest.NewRecorder()
	s.ToolVerifyEmail(rec, httptest.NewRequest("POST", "/api/v1/tools/verify-email", strings.NewReader(`{}`)))
	if rec.Code != 400 {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestToolLinkedInFormatHandler(t *testing.T) {
	s := toolsTestServer(t)
	rec := httptest.NewRecorder()
	s.ToolLinkedInFormat(rec, httptest.NewRequest("POST", "/api/v1/tools/linkedin-format", strings.NewReader(`{"url":"linkedin.com/in/jane-doe?utm_source=newsletter"}`)))
	if rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	var out researchtools.LinkedInFormat
	json.NewDecoder(rec.Body).Decode(&out)
	if out.Formatted != "https://www.linkedin.com/in/jane-doe" {
		t.Fatalf("%+v", out)
	}
}

func TestToolBulkHandlers(t *testing.T) {
	s := toolsTestServer(t)
	rec := httptest.NewRecorder()
	s.ToolBulkStart(rec, httptest.NewRequest("POST", "/api/v1/tools/bulk", strings.NewReader(`{"type":"emails","items":["a@x.com"]}`)))
	if rec.Code != 200 {
		t.Fatalf("start code %d", rec.Code)
	}
	var started map[string]any
	json.NewDecoder(rec.Body).Decode(&started)
	if started["id"].(float64) == 0 {
		t.Fatalf("no job id: %+v", started)
	}
	// The job runs in a background goroutine; wait for it to finish so the
	// test DB is quiet before t.TempDir cleanup removes it.
	deadline := time.Now().Add(2 * time.Second)
	for {
		rec2 := httptest.NewRecorder()
		s.ToolBulkStatus(rec2, withID(httptest.NewRequest("GET", "/api/v1/tools/bulk/1", nil), "1"))
		if rec2.Code != 200 {
			t.Fatalf("status code %d", rec2.Code)
		}
		var job struct {
			Status string `json:"status"`
		}
		json.NewDecoder(rec2.Body).Decode(&job)
		if job.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("job did not complete: %s", job.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

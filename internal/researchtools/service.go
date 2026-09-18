package researchtools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"research-leads/internal/webcheck"
)

// MaxBulkItems caps one queued batch (the spec asks for 10-20 at once).
const MaxBulkItems = 25

// Service bundles the Tools utilities plus the DB used by queued bulk jobs.
type Service struct {
	DB  *sql.DB
	Age *DomainAgeChecker
	// Verify and CheckSite default to the real implementations; fields so
	// tests can stub DNS and HTTP.
	Verify    func(ctx context.Context, email string) EmailVerdict
	CheckSite func(ctx context.Context, domain string) webcheck.Result
}

// NewService wires the production defaults.
func NewService(db *sql.DB, timeout int) *Service {
	return &Service{DB: db, Age: NewDomainAgeChecker(timeout)}
}

// VerifyEmail runs the (possibly stubbed) verifier.
func (s *Service) VerifyEmail(ctx context.Context, email string) EmailVerdict {
	if s.Verify != nil {
		return s.Verify(ctx, email)
	}
	return VerifyEmail(ctx, email, nil)
}

func (s *Service) checkSite(ctx context.Context, domain string) webcheck.Result {
	if s.CheckSite != nil {
		return s.CheckSite(ctx, domain)
	}
	return webcheck.Check(ctx, domain)
}

// BulkJob is one queued batch.
type BulkJob struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"` // emails | urls
	Status    string          `json:"status"`
	Total     int             `json:"total"`
	Completed int             `json:"completed"`
	Results   json.RawMessage `json:"results"`
	CreatedAt string          `json:"created_at"`
}

// StartBulk validates and queues a batch, processing it in the background.
func (s *Service) StartBulk(ctx context.Context, jobType string, items []string) (int64, error) {
	if s.DB == nil {
		return 0, fmt.Errorf("database unavailable")
	}
	if jobType != "emails" && jobType != "urls" {
		return 0, fmt.Errorf("type must be emails or urls")
	}
	clean := []string{}
	for _, it := range items {
		if it = strings.TrimSpace(it); it != "" {
			clean = append(clean, it)
		}
	}
	if len(clean) == 0 {
		return 0, fmt.Errorf("at least one item is required")
	}
	if len(clean) > MaxBulkItems {
		return 0, fmt.Errorf("at most %d items per batch", MaxBulkItems)
	}
	raw, _ := json.Marshal(clean)
	res, err := s.DB.ExecContext(ctx, "INSERT INTO tool_jobs(type,status,total,items,results) VALUES(?,?,?,?,?)", jobType, "queued", len(clean), string(raw), "[]")
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	go s.processBulk(id, jobType, clean)
	return id, nil
}

// processBulk works through the batch sequentially, updating progress after
// each item so the UI can poll. Runs in its own goroutine.
func (s *Service) processBulk(id int64, jobType string, items []string) {
	ctx := context.Background()
	s.DB.ExecContext(ctx, "UPDATE tool_jobs SET status='processing', updated_at=CURRENT_TIMESTAMP WHERE id=?", id)
	results := make([]any, 0, len(items))
	for i, item := range items {
		var r any
		if jobType == "emails" {
			r = s.VerifyEmail(ctx, item)
		} else {
			r = s.checkSite(ctx, item)
		}
		results = append(results, r)
		raw, _ := json.Marshal(results)
		s.DB.ExecContext(ctx, "UPDATE tool_jobs SET completed=?, results=?, updated_at=CURRENT_TIMESTAMP WHERE id=?", i+1, string(raw), id)
	}
	s.DB.ExecContext(ctx, "UPDATE tool_jobs SET status='completed', updated_at=CURRENT_TIMESTAMP WHERE id=?", id)
}

// BulkStatus returns one job with its results.
func (s *Service) BulkStatus(ctx context.Context, id int64) (*BulkJob, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	var j BulkJob
	var results sql.NullString
	var created sql.NullString
	err := s.DB.QueryRowContext(ctx, "SELECT id,type,status,total,completed,results,created_at FROM tool_jobs WHERE id=?", id).
		Scan(&j.ID, &j.Type, &j.Status, &j.Total, &j.Completed, &results, &created)
	if err != nil {
		return nil, fmt.Errorf("job not found")
	}
	j.Results = json.RawMessage(results.String)
	if j.Results == nil {
		j.Results = json.RawMessage("[]")
	}
	j.CreatedAt = created.String
	return &j, nil
}

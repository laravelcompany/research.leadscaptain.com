package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"research-leads/internal/agent"
	"research-leads/internal/api/handlers"
	"research-leads/internal/api/middleware"
	"research-leads/internal/emailvalidator"
	"research-leads/internal/events"
)

func Router(db *sql.DB, bus *events.Bus, engine *agent.Engine, corsOrigins, apiKey string, verifier *emailvalidator.Client, auth *handlers.AuthConfig) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, req)
			_ = start
		})
	})
	origins := []string{"*"}
	if strings.TrimSpace(corsOrigins) != "" && strings.TrimSpace(corsOrigins) != "*" {
		origins = nil
		for _, o := range strings.Split(corsOrigins, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
		if len(origins) == 0 {
			origins = []string{"*"}
		}
	}
	r.Use(cors.Handler(cors.Options{AllowedOrigins: origins, AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"}, AllowedHeaders: []string{"*"}}))
	r.Use(middleware.APIKey(apiKey, auth.Secret))
	r.Use(middleware.Session(auth.User, auth.Pass, auth.Secret, apiKey))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			fmt.Printf("{\"time\":\"%s\",\"level\":\"DEBUG\",\"msg\":\"request\",\"method\":\"%s\",\"path\":\"%s\",\"query\":\"%s\"}\n", time.Now().Format(time.RFC3339), req.Method, req.URL.Path, req.URL.RawQuery)
			next.ServeHTTP(w, req)
		})
	})
	s := &handlers.Server{DB: db, Bus: bus, Engine: engine, Auth: auth}
	if verifier != nil {
		s.VerifyEmail = func(ctx context.Context, email string) (string, error) {
			res, err := verifier.Verify(ctx, email)
			return res.Status, err
		}
	}
	r.Get("/health", s.Health)
	r.Get("/health/live", s.Health)
	r.Get("/health/ready", s.Health)
	r.Get("/metrics", s.Metrics)
	r.Post("/api/v1/auth/login", s.Login)
	r.Get("/api/v1/auth/me", s.Me)
	r.Post("/api/v1/auth/logout", s.Logout)
	r.Get("/api/v1/events", s.Events)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/objectives", s.Objectives)
		r.Post("/objectives", s.Objectives)
		r.Get("/objectives/{id}", s.ObjectiveByID)
		r.Delete("/objectives/{id}", s.ObjectiveByID)
		r.Post("/objectives/{id}/start", s.StartObjective)
		r.Post("/objectives/{id}/pause", s.PauseObjective)
		r.Post("/objectives/{id}/resume", s.ResumeObjective)
		r.Post("/objectives/{id}/stop", s.StopObjective)
		r.Get("/leads", s.Leads)
		r.Delete("/leads", s.Leads)
		r.Get("/stats", s.Stats)
		r.Get("/stats/leads", s.Stats)
		r.Get("/audit", s.Audit)
		r.Get("/tasks", s.Tasks)
		r.Get("/lists", s.Lists)
		r.Post("/lists", s.Lists)
		r.Get("/leads/export", s.LeadsExport)
		r.Get("/iterations", s.Iterations)
		r.Get("/search-history", s.SearchHistory)
		r.Post("/leads/search", s.AdvancedSearch)
		r.Post("/leads/{id}/status", s.LeadStatus)
		r.Get("/leads/{id}/score-explanation", s.LeadScoreExplain)
		r.Get("/tags", s.Tags)
		r.Post("/tags", s.Tags)
		r.Get("/analytics/funnel", s.AnalyticsFunnel)
		r.Get("/segments", s.Segments)
		r.Post("/segments", s.Segments)
		r.Get("/segments/{id}/leads", s.SegmentLeads)
		r.Get("/saved-searches", s.SavedSearches)
		r.Post("/saved-searches", s.SavedSearches)
		r.Post("/bulk/{type}", s.BulkOperation)
		r.Get("/bulk/{id}", s.BulkStatus)
		r.Get("/research/queue", s.ResearchQueue)
		r.Post("/research/queue", s.ResearchQueue)
		r.Get("/imports", s.Imports)
		r.Post("/imports", s.Imports)
		r.Get("/exports/profiles", s.ExportProfiles)
		r.Post("/exports/profiles", s.ExportProfiles)
		r.Post("/analytics/gap", s.GapAnalysis)
		r.Get("/leads/stale", s.StaleLeads)
		r.Get("/leads/{id}/similar", s.SimilarLeads)
	})
	staticDir := "frontend/dist"
	for _, p := range []string{"frontend/dist", "./frontend/dist", "static", "./static", "/app/static"} {
		if _, err := os.Stat(p); err == nil {
			staticDir = p
			break
		}
	}
	if _, err := os.Stat(staticDir); err == nil {
		r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.URL.Path, "/api/") || strings.HasPrefix(req.URL.Path, "/health") || strings.HasPrefix(req.URL.Path, "/metrics") {
				http.NotFound(w, req)
				return
			}
			path := filepath.Join(staticDir, filepath.Clean(req.URL.Path))
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				http.ServeFile(w, req, path)
				return
			}
			http.ServeFile(w, req, filepath.Join(staticDir, "index.html"))
		})
	}
	return r
}

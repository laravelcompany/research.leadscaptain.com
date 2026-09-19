package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"research-leads/internal/agent"
	"research-leads/internal/ai"
	"research-leads/internal/api"
	"research-leads/internal/api/handlers"
	"research-leads/internal/api/middleware"
	"research-leads/internal/companies"
	"research-leads/internal/companyreg"
	"research-leads/internal/config"
	"research-leads/internal/db"
	"research-leads/internal/emailvalidator"
	"research-leads/internal/events"
	"research-leads/internal/leadscaptain"
	"research-leads/internal/observability"
	"research-leads/internal/researchtools"
	"research-leads/internal/tools"
)

func main() {
	cfg := config.Load()
	logger := observability.NewLogger(cfg.LogLevel)
	os.MkdirAll("data", 0755)
	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	bus := events.New()
	aiClient := ai.New(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel, cfg.AITimeout)
	lc := leadscaptain.New(cfg.LCBaseURL, cfg.LCAPIToken, cfg.APITimeout)
	var prober *researchtools.SMTPProber
	if cfg.EmailSMTPProbe {
		prober = &researchtools.SMTPProber{Helo: cfg.EmailSMTPHelo, From: cfg.EmailSMTPFrom, Timeout: 8 * time.Second}
	} else {
		logger.Info("smtp email probing disabled", "env", "EMAIL_SMTP_PROBE")
	}
	localVerifier := &researchtools.Verifier{Prober: prober}
	ev := emailvalidator.New(cfg.EmailValURL, cfg.EmailValKey, cfg.APITimeout, localVerifier)
	reg := tools.NewRegistry()
	reg.Register(tools.NewSearch(lc))
	reg.Register(tools.NewVerify(ev))
	reg.Register(tools.NewStats(database))
	reg.Register(tools.NewGetLead(database))
	reg.Register(tools.NewListLeads(database))
	reg.Register(tools.NewScoreLead(database))
	reg.Register(tools.NewExportLeads(database, cfg.ExportDir))
	reg.Register(tools.NewCheckWebsite())
	reg.Register(tools.NewCheckDomain())
	reg.Register(tools.NewCompanyLookup(companyreg.New(cfg.CHAPIKey, cfg.APITimeout)))
	compSvc := &companies.Service{
		DB:           database,
		Autocomplete: companies.NewAutocomplete(cfg.APITimeout),
		Clearbit:     companies.NewClearbit(cfg.ClearbitKey, cfg.APITimeout),
		Registry:     companyreg.New(cfg.CHAPIKey, cfg.APITimeout),
	}
	reg.Register(tools.NewFindEmail(ev))
	engine := agent.New(database, aiClient, reg, bus, logger, cfg.MaxIterations)
	engine.Recover(context.Background())
	authCfg := &handlers.AuthConfig{ClientID: cfg.LinkedInClientID, ClientSecret: cfg.LinkedInClientSecret, RedirectURL: cfg.LinkedInRedirectURL, IssuerURL: cfg.LinkedInIssuerURL, Scopes: strings.Fields(cfg.LinkedInScopes), Secret: middleware.SessionSecret(cfg.SessionSecret)}
	if err := authCfg.ValidationError(); err != nil {
		logger.Warn("LinkedIn authentication configuration incomplete", "error", err)
	} else {
		logger.Info("LinkedIn authentication enabled")
	}
	toolsSvc := researchtools.NewService(database, cfg.APITimeout)
	// The Tools page verifies through the same shared verifier (SMTP probe
	// included) so its verdicts match what the lead pipeline records.
	toolsSvc.Verify = localVerifier.Verify
	handler := api.Router(database, bus, engine, cfg.CORSOrigins, cfg.AppAPIKey, ev, authCfg, compSvc, toolsSvc)
	srv := &http.Server{Addr: cfg.Host + ":" + cfg.Port, Handler: handler}
	go func() {
		logger.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

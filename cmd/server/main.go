package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"research-leads/internal/agent"
	"research-leads/internal/ai"
	"research-leads/internal/api"
	"research-leads/internal/api/handlers"
	"research-leads/internal/api/middleware"
	"research-leads/internal/companyreg"
	"research-leads/internal/config"
	"research-leads/internal/db"
	"research-leads/internal/emailvalidator"
	"research-leads/internal/events"
	"research-leads/internal/leadscaptain"
	"research-leads/internal/observability"
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
	ev := emailvalidator.New(cfg.EmailValURL, cfg.EmailValKey, cfg.APITimeout)
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
	reg.Register(tools.NewFindEmail(ev))
	engine := agent.New(database, aiClient, reg, bus, logger, cfg.MaxIterations)
	engine.Recover(context.Background())
	authCfg := &handlers.AuthConfig{User: cfg.AuthUser, Pass: cfg.AuthPass, Secret: middleware.SessionSecret(cfg.AuthUser, cfg.AuthPass, cfg.SessionSecret)}
	if authCfg.Enabled() {
		logger.Info("ui auth enabled", "user", cfg.AuthUser)
	}
	handler := api.Router(database, bus, engine, cfg.CORSOrigins, cfg.AppAPIKey, ev, authCfg)
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

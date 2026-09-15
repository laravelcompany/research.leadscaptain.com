package config

import (
	"os"
	"strconv"
)

type Config struct {
	Env           string
	Host          string
	Port          string
	DatabasePath  string
	AIBaseURL     string
	AIModel       string
	AIAPIKey      string
	LCBaseURL     string
	LCAPIToken    string
	EmailValURL   string
	EmailValKey   string
	MaxIterations int
	MaxTasks      int
	AITimeout     int
	APITimeout    int
	MaxConcurrent int
	LogLevel      string
	AppAPIKey     string
	CORSOrigins   string
	CHAPIKey      string
	ExportDir     string
	AuthUser      string
	AuthPass      string
	SessionSecret string
}

func Load() Config {
	return Config{
		Env:           env("APP_ENV", "development"),
		Host:          env("APP_HOST", "0.0.0.0"),
		Port:          env("APP_PORT", "8080"),
		DatabasePath:  env("DATABASE_PATH", "/tmp/research-leads/leads.db"),
		AIBaseURL:     env("AI_BASE_URL", "https://ai.izdrail.com"),
		AIModel:       env("AI_MODEL", "gemma4:e2b"),
		AIAPIKey:      os.Getenv("AI_API_KEY"),
		LCBaseURL:     env("LEADSCAPTAIN_BASE_URL", "https://api.leadscaptain.com"),
		LCAPIToken:    os.Getenv("LEADSCAPTAIN_API_TOKEN"),
		EmailValURL:   os.Getenv("EMAIL_VALIDATION_URL"),
		EmailValKey:   os.Getenv("EMAIL_VALIDATION_API_KEY"),
		MaxIterations: envInt("AGENT_MAX_ITERATIONS", 50),
		MaxTasks:      envInt("AGENT_MAX_TASKS", 500),
		AITimeout:     envInt("AI_REQUEST_TIMEOUT", 120),
		APITimeout:    envInt("API_REQUEST_TIMEOUT", 60),
		MaxConcurrent: envInt("MAX_CONCURRENT_TASKS", 3),
		LogLevel:      env("LOG_LEVEL", "info"),
		AppAPIKey:     os.Getenv("APP_API_KEY"),
		CORSOrigins:   env("CORS_ALLOWED_ORIGINS", "*"),
		CHAPIKey:      os.Getenv("COMPANIES_HOUSE_API_KEY"),
		ExportDir:     env("EXPORT_DIR", "/tmp/research-leads/exports"),
		AuthUser:      os.Getenv("AUTH_USERNAME"),
		AuthPass:      os.Getenv("AUTH_PASSWORD"),
		SessionSecret: os.Getenv("AUTH_SESSION_SECRET"),
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}

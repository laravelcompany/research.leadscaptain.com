package config

import (
	"os"
	"strconv"
	"strings"
)

const emailValidatorDefaultURL = "https://validation.laravelmail.com"

type Config struct {
	Env                  string
	Host                 string
	Port                 string
	DatabasePath         string
	AIBaseURL            string
	AIModel              string
	AIAPIKey             string
	LCBaseURL            string
	LCAPIToken           string
	EmailValURL          string
	EmailValKey          string
	EmailSMTPProbe       bool
	EmailSMTPHelo        string
	EmailSMTPFrom        string
	MaxIterations        int
	MaxTasks             int
	AITimeout            int
	APITimeout           int
	MaxConcurrent        int
	LogLevel             string
	AppAPIKey            string
	CORSOrigins          string
	CHAPIKey             string
	ClearbitKey          string
	ExportDir            string
	LinkedInClientID     string
	LinkedInClientSecret string
	LinkedInRedirectURL  string
	LinkedInIssuerURL    string
	LinkedInScopes       string
	SessionSecret        string
}

func Load() Config {
	return Config{
		Env:                  env("APP_ENV", "development"),
		Host:                 env("APP_HOST", "0.0.0.0"),
		Port:                 env("APP_PORT", "8080"),
		DatabasePath:         env("DATABASE_PATH", "/tmp/research-leads/leads.db"),
		AIBaseURL:            env("AI_BASE_URL", "https://ai.izdrail.com"),
		AIModel:              env("AI_MODEL", "gemma4:e2b"),
		AIAPIKey:             os.Getenv("AI_API_KEY"),
		LCBaseURL:            env("LEADSCAPTAIN_BASE_URL", "https://api.leadscaptain.com"),
		LCAPIToken:           os.Getenv("LEADSCAPTAIN_API_TOKEN"),
		EmailValURL:          env("EMAIL_VALIDATION_URL", emailValidatorDefaultURL),
		EmailValKey:          os.Getenv("EMAIL_VALIDATION_API_KEY"),
		EmailSMTPProbe:       envBool("EMAIL_SMTP_PROBE", true),
		EmailSMTPHelo:        env("EMAIL_SMTP_HELO", "localhost.localdomain"),
		EmailSMTPFrom:        env("EMAIL_SMTP_FROM", "probe@localhost"),
		MaxIterations:        envInt("AGENT_MAX_ITERATIONS", 50),
		MaxTasks:             envInt("AGENT_MAX_TASKS", 500),
		AITimeout:            envInt("AI_REQUEST_TIMEOUT", 120),
		APITimeout:           envInt("API_REQUEST_TIMEOUT", 60),
		MaxConcurrent:        envInt("MAX_CONCURRENT_TASKS", 3),
		LogLevel:             env("LOG_LEVEL", "info"),
		AppAPIKey:            os.Getenv("APP_API_KEY"),
		CORSOrigins:          env("CORS_ALLOWED_ORIGINS", "*"),
		CHAPIKey:             os.Getenv("COMPANIES_HOUSE_API_KEY"),
		ClearbitKey:          os.Getenv("CLEARBIT_API_KEY"),
		ExportDir:            env("EXPORT_DIR", "/tmp/research-leads/exports"),
		LinkedInClientID:     os.Getenv("LINKEDIN_CLIENT_ID"),
		LinkedInClientSecret: os.Getenv("LINKEDIN_CLIENT_SECRET"),
		LinkedInRedirectURL:  os.Getenv("LINKEDIN_REDIRECT_URL"),
		LinkedInIssuerURL:    env("LINKEDIN_ISSUER_URL", "https://www.linkedin.com/oauth"),
		LinkedInScopes:       env("LINKEDIN_SCOPES", "openid profile email"),
		SessionSecret:        os.Getenv("AUTH_SESSION_SECRET"),
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func envBool(k string, d bool) bool {
	v := strings.ToLower(os.Getenv(k))
	if v == "" {
		return d
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}

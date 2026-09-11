package config

import (
	"os"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
// Never hardcode secrets — all sensitive values must come from env.
type Config struct {
	// Database DSN for MySQL connection
	DBDSN string

	// HTTP server listen address
	ServerAddr string

	// JWT signing secret — REQUIRED
	JWTSecret string

	// Log level: debug, info, warn, error
	LogLevel string

	// Environment: development, staging, production
	Env string
}

// Load reads configuration from environment variables.
// Panics if required variables (JWT_SECRET) are missing.
func Load() *Config {
	cfg := &Config{
		DBDSN:      getEnv("DB_DSN", "root:@tcp(localhost:3306)/wertku?parseTime=true&loc=UTC"),
		ServerAddr: getEnv("SERVER_ADDR", ":8080"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),
		Env:        getEnv("ENV", "development"),
	}

	if strings.TrimSpace(cfg.JWTSecret) == "" {
		// In development, use a default secret with a warning.
		// In production this must be set explicitly.
		if cfg.Env == "development" {
			cfg.JWTSecret = "wertku-dev-secret-change-in-production"
		} else {
			panic("JWT_SECRET environment variable is required")
		}
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

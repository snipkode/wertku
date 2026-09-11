package config

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Config holds all application configuration.
// Values are loaded from a YAML file, with environment variable overrides.
//
// Load order (last wins):
//  1. config.yaml  (or path set by CONFIG_FILE env var)
//  2. Environment variable overrides
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Log      LogConfig      `yaml:"log"`
	Env      string         `yaml:"env"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
}

type AuthConfig struct {
	JWTSecret string `yaml:"jwt_secret"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

// defaults returns a Config pre-filled with safe default values.
func defaults() *Config {
	return &Config{
		Server:   ServerConfig{Addr: ":8080"},
		Database: DatabaseConfig{DSN: "root:@tcp(localhost:3306)/wertku?parseTime=true&loc=UTC"},
		Auth:     AuthConfig{JWTSecret: ""},
		Log:      LogConfig{Level: "info"},
		Env:      "development",
	}
}

// Load reads configuration from a YAML file, then applies environment
// variable overrides. This allows secrets to stay out of YAML files
// in production (e.g., set JWT_SECRET as an env var in CI/CD).
//
// YAML file path resolution (first match wins):
//  1. CONFIG_FILE env var
//  2. ./config.yaml
//  3. /etc/wertku/config.yaml
func Load() *Config {
	cfg := defaults()

	// Determine config file path
	path := configFilePath()

	if path != "" {
		if err := loadYAML(path, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "config: warning: %v\n", err)
		}
	}

	// Environment variable overrides — useful for secrets in production
	applyEnvOverrides(cfg)

	// Validate required fields
	validate(cfg)

	return cfg
}

// loadYAML reads and unmarshals a YAML config file into cfg.
func loadYAML(path string, cfg *Config) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open config file %q: %w", path, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true) // error on unknown keys
	if err := dec.Decode(cfg); err != nil {
		return fmt.Errorf("cannot parse config file %q: %w", path, err)
	}
	return nil
}

// applyEnvOverrides applies environment variable overrides on top of YAML values.
// An env var overrides the YAML value if the env var is non-empty.
//
//	DB_DSN        → database.dsn
//	SERVER_ADDR   → server.addr
//	JWT_SECRET    → auth.jwt_secret
//	LOG_LEVEL     → log.level
//	ENV           → env
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("SERVER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("ENV"); v != "" {
		cfg.Env = v
	}
}

// validate checks that required configuration values are present.
func validate(cfg *Config) {
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		if cfg.Env == "development" {
			cfg.Auth.JWTSecret = "wertku-dev-secret-change-in-production"
			fmt.Fprintln(os.Stderr, "config: WARNING: using default JWT secret — set auth.jwt_secret in config.yaml or JWT_SECRET env var")
		} else {
			panic("config: auth.jwt_secret is required in non-development environments")
		}
	}
}

// configFilePath returns the path to the config file to use.
func configFilePath() string {
	// 1. Explicit env override
	if v := os.Getenv("CONFIG_FILE"); v != "" {
		return v
	}
	// 2. Local config.yaml
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	// 3. System-wide config
	if _, err := os.Stat("/etc/wertku/config.yaml"); err == nil {
		return "/etc/wertku/config.yaml"
	}
	return ""
}

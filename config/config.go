// config.go
package config

import (
	"errors"
	"os"
)

// Config holds runtime configuration for the app.
type Config struct {
	Port         string
	BaseHost     string
	DatabasePath string
	AppEnv  string // "development" | "production"
	LogLevel string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:         getenv("PORT", "8080"),
		BaseHost:     os.Getenv("BASE_HOST"),
		DatabasePath: getenv("DATABASE_PATH", "data/db.sqlite3"),
		AppEnv:       getenv("APP_ENV", "production"),
		LogLevel:     getenv("LOG_LEVEL", "info"),
	}

	// Required validations
	if cfg.BaseHost == "" {
		return nil, errors.New("BASE_HOST is required")
	}

	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
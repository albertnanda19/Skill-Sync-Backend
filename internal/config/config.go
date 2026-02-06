package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
}

type AppConfig struct {
	AppName     string
	Environment string
	HTTPPort    string
}

type DatabaseConfig struct {
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBSSLMode  string
}

var errMissingRequiredEnv = errors.New("missing required environment variables")

func Load() (Config, error) {
	cfg := Config{}

	var missing []string
	req := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg.App = AppConfig{
		AppName:     req("APP_NAME"),
		Environment: req("APP_ENV"),
		HTTPPort:    req("HTTP_PORT"),
	}

	cfg.Database = DatabaseConfig{
		DBHost:     req("DB_HOST"),
		DBPort:     req("DB_PORT"),
		DBName:     req("DB_NAME"),
		DBUser:     req("DB_USER"),
		DBPassword: req("DB_PASSWORD"),
		DBSSLMode:  req("DB_SSL_MODE"),
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("%w: %s", errMissingRequiredEnv, strings.Join(missing, ", "))
	}

	return cfg, nil
}

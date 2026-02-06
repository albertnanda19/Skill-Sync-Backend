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
	_ = loadDotEnvIfPresent(".env")

	cfg := Config{}

	var missing []string
	req := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}
	opt := func(key string) string {
		return strings.TrimSpace(os.Getenv(key))
	}

	cfg.App = AppConfig{
		AppName:     req("APP_NAME"),
		Environment: req("APP_ENV"),
		HTTPPort:    req("HTTP_PORT"),
	}

	cfg.Database = DatabaseConfig{
		DBHost:     opt("DB_HOST"),
		DBPort:     opt("DB_PORT"),
		DBName:     opt("DB_NAME"),
		DBUser:     opt("DB_USER"),
		DBPassword: opt("DB_PASSWORD"),
		DBSSLMode:  opt("DB_SSL_MODE"),
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("%w: %s", errMissingRequiredEnv, strings.Join(missing, ", "))
	}

	return cfg, nil
}

func loadDotEnvIfPresent(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(b), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		val := strings.TrimSpace(v)
		_ = os.Setenv(key, val)
	}

	return nil
}

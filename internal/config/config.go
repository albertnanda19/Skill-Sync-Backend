package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	JWT      JWTConfig

	InternalToken string

	SearchFreshnessMinutes int
	ScraperBaseURL         string
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

	RunSeeders bool

	ConnectTimeout        time.Duration
	PoolMaxConns          int32
	PoolMinConns          int32
	PoolMaxConnLifetime   time.Duration
	PoolMaxConnIdleTime   time.Duration
	PoolHealthCheckPeriod time.Duration
}

type JWTConfig struct {
	AccessSecret     string
	RefreshSecret    string
	AccessExpiresIn  time.Duration
	RefreshExpiresIn time.Duration
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
	reqDuration := func(key string) time.Duration {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			missing = append(missing, key)
			return 0
		}
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			missing = append(missing, key)
			return 0
		}
		return d
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

		RunSeeders: optBool("DB_RUN_SEEDERS"),

		ConnectTimeout:        optDuration("DB_CONNECT_TIMEOUT"),
		PoolMaxConns:          optInt32("DB_POOL_MAX_CONNS"),
		PoolMinConns:          optInt32("DB_POOL_MIN_CONNS"),
		PoolMaxConnLifetime:   optDuration("DB_POOL_MAX_CONN_LIFETIME"),
		PoolMaxConnIdleTime:   optDuration("DB_POOL_MAX_CONN_IDLE_TIME"),
		PoolHealthCheckPeriod: optDuration("DB_POOL_HEALTHCHECK_PERIOD"),
	}

	cfg.JWT = JWTConfig{
		AccessSecret:     req("JWT_ACCESS_SECRET"),
		RefreshSecret:    req("JWT_REFRESH_SECRET"),
		AccessExpiresIn:  reqDuration("JWT_ACCESS_EXPIRES_IN"),
		RefreshExpiresIn: reqDuration("JWT_REFRESH_EXPIRES_IN"),
	}

	cfg.InternalToken = req("INTERNAL_TOKEN")

	cfg.SearchFreshnessMinutes = optInt("SEARCH_FRESHNESS_MINUTES", 30)
	cfg.ScraperBaseURL = opt("SCRAPER_BASE_URL")

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("%w: %s", errMissingRequiredEnv, strings.Join(missing, ", "))
	}

	return cfg, nil
}

func optInt(key string, defaultVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	if v <= 0 {
		return defaultVal
	}
	return v
}

func optInt32(key string) int32 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0
	}
	if v <= 0 {
		return 0
	}
	return int32(v)
}

func optDuration(key string) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0
	}
	if d <= 0 {
		return 0
	}
	return d
}

func optBool(key string) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return v
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

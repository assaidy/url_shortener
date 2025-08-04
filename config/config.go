package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

var (
	ServerAddr = getEnvString("SERVER_ADDR", "localhost:8080")
	SecretKey  = getEnvString("SECRET_KEY")

	PgHost     = getEnvString("PG_HOST", "localhost")
	PgPort     = getEnvInt("PG_PORT", 5432)
	PgUser     = getEnvString("PG_USER", "postgres")
	PgPassword = getEnvString("PG_PASSWORD")
	PgName     = getEnvString("PgName", "url_shortener")
	PgSSL      = getEnvString("PG_SSL_MODE", "disable")

	CacheTTL   = 10 * time.Minute
	ValkeyAddr = getEnvString("VALKEY_ADDR", "localhost:6379")

	JwtTokenExpiration               = 7 * 24 * time.Hour // 7 days
	RandomUrlCollisionRetries        = 5
	RedirectionRateLimitMaxPerWindow = 20
	RedirectionRateLimitWindow       = 1 * time.Minute
)

func getEnvInt(key string, defaultValue ...int) int {
	if value, ok := os.LookupEnv(key); ok {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		slog.Error("invalid int env var", "key", key, "value", value)
		os.Exit(1)
	}
	if len(defaultValue) == 0 {
		slog.Error("env var not found", "key", key)
		os.Exit(1)
	}
	return defaultValue[0]
}

func getEnvString(key string, defaultValue ...string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	if len(defaultValue) == 0 {
		slog.Error("env var not found", "key", key)
		os.Exit(1)
	}
	return defaultValue[0]
}

package postgres_db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/assaidy/url_shortener/config"
	_ "github.com/lib/pq"
)

var DB *sql.DB

func init() {
	conn, err := sql.Open("postgres", fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.PgHost, config.PgPort, config.PgUser, config.PgPassword, config.PgName, config.PgSSL,
	))
	if err != nil {
		slog.Error("error connecting to postgres db", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		slog.Error("error pinging postgres db", "err", err)
		os.Exit(1)
	}

	DB = conn
}

package db

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

var Connection *sql.DB

func init() {
	db, err := sql.Open("postgres", fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.PgHost, config.PgPort, config.PgUser, config.PgPassword, config.PgName, config.PgSSL,
	))
	if err != nil {
		slog.Error("error connecting to postgres db", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		slog.Error("error pinging postgres db", "err", err)
		os.Exit(1)
	}

	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(1 * time.Minute)

	Connection = db
}

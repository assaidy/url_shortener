package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/assaidy/moon"
	_ "github.com/lib/pq"
)

var _ moon.Service = (*Service)(nil)

type Service struct {
	url  string
	pool *sql.DB
}

func New(url string) moon.ServiceConstructor[*Service] {
	return func(i moon.Injected) *Service {
		return &Service{url: url}
	}
}

func (me *Service) GetDb() *sql.DB {
	return me.pool
}

// Name implements [moon.Service].
func (me *Service) Name() string {
	return "postgres"
}

// Start implements [moon.Service].
func (me *Service) Start() error {
	pool, err := sql.Open("postgres", me.url)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		return err
	}

	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(5 * time.Minute)
	pool.SetConnMaxIdleTime(1 * time.Minute)

	me.pool = pool
	return nil
}

// Stop implements [moon.Service].
func (me *Service) Stop() error {
	return me.pool.Close()
}

// IsReady implements [moon.Service].
func (me *Service) IsReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return me.pool.PingContext(ctx) == nil
}

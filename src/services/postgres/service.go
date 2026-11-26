package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/assaidy/moon"
	_ "github.com/lib/pq"
)

var _ moon.Service = (*impl)(nil)

type impl struct {
	url  string
	pool *sql.DB
}

func New(url string) moon.ServiceConstructor {
	return func(dependencies *moon.DependencyContainer) moon.Service {
		return &impl{url: url}
	}
}

// Name implements [moon.Service].
func (me *impl) Name() string {
	return "postgres"
}

// Start implements [moon.Service].
func (me *impl) Start(deps *moon.DependencyContainer) error {
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

	deps.Set(me.pool)

	return nil
}

// Stop implements [moon.Service].
func (me *impl) Stop(deps *moon.DependencyContainer) error {
	deps.Delete[*sql.DB]()
	return me.pool.Close()
}

// IsReady implements [moon.Service].
func (me *impl) IsReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return me.pool.PingContext(ctx) == nil
}

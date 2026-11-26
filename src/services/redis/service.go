package redis

import (
	"context"
	"time"

	"github.com/assaidy/moon"
	"github.com/redis/go-redis/v9"
)

var _ moon.Service = (*impl)(nil)

type impl struct {
	address string
	db      int
	client  *redis.Client
}

// New returns a Service for address. An optional db selects the Redis database,
// defaulting to 0.
func New(address string, db ...int) moon.ServiceConstructor {
	index := 0
	if len(db) > 0 {
		index = db[0]
	}
	return func(dependencies *moon.DependencyContainer) moon.Service {
		return &impl{address: address, db: index}
	}
}

// Name implements [moon.Service].
func (me *impl) Name() string {
	return "redis"
}

// Start implements [moon.Service].
func (me *impl) Start(deps *moon.DependencyContainer) error {
	client := redis.NewClient(&redis.Options{
		Addr: me.address,
		DB:   me.db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return err
	}

	me.client = client

	deps.Set(client)

	return nil
}

// Stop implements [moon.Service].
func (me *impl) Stop(deps *moon.DependencyContainer) error {
	deps.Delete[*redis.Client]()
	return me.client.Close()
}

// IsReady implements [moon.Service].
func (me *impl) IsReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return me.client.Ping(ctx).Err() == nil
}

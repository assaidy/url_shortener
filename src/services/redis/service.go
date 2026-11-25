package redis

import (
	"context"
	"time"

	"github.com/assaidy/moon"
	"github.com/redis/go-redis/v9"
)

var _ moon.Service = (*Service)(nil)

type Service struct {
	address string
	db      int
	client  *redis.Client
}

// New returns a Service for address. An optional db selects the Redis database,
// defaulting to 0.
func New(address string, db ...int) moon.ServiceConstructor[*Service] {
	index := 0
	if len(db) > 0 {
		index = db[0]
	}
	return func(i moon.Injected) *Service {
		return &Service{address: address, db: index}
	}
}

func (me *Service) GetClient() *redis.Client {
	return me.client
}

// Name implements [moon.Service].
func (me *Service) Name() string {
	return "redis"
}

// Start implements [moon.Service].
func (me *Service) Start() error {
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
	return nil
}

// Stop implements [moon.Service].
func (me *Service) Stop() error {
	return me.client.Close()
}

// IsReady implements [moon.Service].
func (me *Service) IsReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return me.client.Ping(ctx).Err() == nil
}

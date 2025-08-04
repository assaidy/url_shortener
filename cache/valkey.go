package cache

import (
	"log/slog"
	"os"

	"github.com/assaidy/url_shortener/config"
	"github.com/valkey-io/valkey-go"
)

var Valkey valkey.Client

func init() {
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{config.ValkeyAddr}})
	if err != nil {
		slog.Error("error connecting to valkey server", "err", err)
		os.Exit(1)
	}

	Valkey = client
}

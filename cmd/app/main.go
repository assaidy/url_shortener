package main

import (
	"log/slog"
	"os"

	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/internals/routes"
	auth_service "github.com/assaidy/url_shortener/internals/services/auth"
	postgres_service "github.com/assaidy/url_shortener/internals/services/postgres"
	redirection_service "github.com/assaidy/url_shortener/internals/services/redirection"
	redis_service "github.com/assaidy/url_shortener/internals/services/redis"
	shortening_service "github.com/assaidy/url_shortener/internals/services/shortening"
	"github.com/charmbracelet/log"
	_ "github.com/joho/godotenv/autoload"
)

// TODO: add rate limiting

func main() {
	logger := slog.New(log.NewWithOptions(os.Stderr, log.Options{ReportTimestamp: true}))

	app := moon.New(
		moon.WithListenAddress(":3000"),
		moon.WithLogger(logger.WithGroup("moon")),
		moon.WithRequestLogging(true),
	)

	app.AddDependency(logger)

	// TODO: use service interfaces instead of pointers.
	// the interface shouldn't include [moon.Service] impl methods
	// in order to hide them from users of the service.
	// still, assert [moon.Service] on the service impl internally.
	app.AddService(postgres_service.New(moon.MustGetEnv("POSTGRES_URL")))
	app.AddService(redis_service.New(moon.MustGetEnv("REDIS_ADDRESS")))
	app.AddService(auth_service.New())
	app.AddService(shortening_service.New())
	app.AddService(redirection_service.New())

	routes.Map(app)

	err := app.RunUntilSignal()
	moon.Assert(err == nil, err)
}

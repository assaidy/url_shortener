package main

import (
	"log/slog"
	"os"

	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/src/routes"
	auth_service "github.com/assaidy/url_shortener/src/services/auth"
	postgres_service "github.com/assaidy/url_shortener/src/services/postgres"
	redirection_service "github.com/assaidy/url_shortener/src/services/redirection"
	redis_service "github.com/assaidy/url_shortener/src/services/redis"
	shortening_service "github.com/assaidy/url_shortener/src/services/shortening"
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

	app.AddService(postgres_service.New(moon.MustGetEnv("POSTGRES_URL")))
	app.AddService(redis_service.New(moon.MustGetEnv("REDIS_ADDRESS")))
	app.AddService(auth_service.New())
	app.AddService(shortening_service.New())
	app.AddService(redirection_service.New())

	routes.Map(app)

	err := app.RunUntilSignal()
	moon.Assert(err == nil, err)
}

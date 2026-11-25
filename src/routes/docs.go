package routes

import (
	"github.com/assaidy/moon"
	"github.com/assaidy/moon/handlers/swagger"
	"github.com/assaidy/url_shortener/src/specs"
)

func mapDocs(app *moon.App) {
	app.MapGet("/swagger/*", swagger.New(
		swagger.WithTitle("URL Shortener API"),
		swagger.WithVersion("0.1.0"),
		swagger.WithDescription("API for managing short links and user accounts."),
		swagger.WithComponents(specs.Components),
	),
		moon.WithApiSpec(specs.Swagger),
	)
}

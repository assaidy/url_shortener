package routes

import (
	"github.com/assaidy/moon"
	"github.com/assaidy/moon/handlers/pagination"
	"github.com/assaidy/url_shortener/src/handlers"
	"github.com/assaidy/url_shortener/src/specs"
)

func mapShortening(app *moon.App) {
	app.MapPost("/urls", handlers.CreateUrl,
		moon.WithMiddlewares(handlers.HasSession, handlers.HasCsrf),
		moon.WithApiSpec(specs.CreateUrl),
	)
	app.MapGet("/urls", handlers.GetUrls,
		moon.WithMiddlewares(
			handlers.HasSession,
			pagination.New(
				pagination.WithPageQueryKey(""),
				pagination.WithOffsetQueryKey(""),
				pagination.WithSortQueryKey("sort"),
				pagination.WithAllowedSortKeys([]string{"title", "url", "alias", "created_at"}),
				pagination.WithDefaultSort([]pagination.Sort{{Key: "created_at", Order: pagination.OrderDescending}}),
			),
		),
		moon.WithApiSpec(specs.GetUrls),
	)
	app.MapGet("/urls/:id", handlers.GetOneUrl,
		moon.WithMiddlewares(handlers.HasSession),
		moon.WithApiSpec(specs.GetOneUrl),
	)
	app.MapPut("/urls/:id", handlers.EditUrl,
		moon.WithMiddlewares(handlers.HasSession, handlers.HasCsrf),
		moon.WithApiSpec(specs.EditUrl),
	)
	app.MapDelete("/urls/:id", handlers.DeleteUrl,
		moon.WithMiddlewares(handlers.HasSession, handlers.HasCsrf),
		moon.WithApiSpec(specs.DeleteUrl),
	)
}

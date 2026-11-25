package routes

import (
	"github.com/assaidy/moon"
	"github.com/assaidy/moon/handlers/pagination"
	"github.com/assaidy/url_shortener/internals/handlers"
	"github.com/assaidy/url_shortener/internals/specs"
)

// redirectionPagination is shared by both redirection listing routes, which use
// the same pagination and sorting policy.
var redirectionPagination = pagination.New(
	pagination.WithPageQueryKey(""),
	pagination.WithOffsetQueryKey(""),
	pagination.WithSortQueryKey("sort"),
	pagination.WithAllowedSortKeys([]string{"created_at", "os", "platform"}),
	pagination.WithDefaultSort([]pagination.Sort{{Key: "created_at", Order: pagination.OrderDescending}}),
)

func mapRedirection(app *moon.App) {
	app.MapGet("/r/:alias", handlers.Redirect,
		moon.WithApiSpec(specs.Redirect),
	)
	app.MapGet("/redirections", handlers.GetRedirections,
		moon.WithMiddlewares(handlers.HasSession, redirectionPagination),
		moon.WithApiSpec(specs.GetRedirections),
	)
	app.MapGet("/redirections/:id", handlers.GetOneRedirection,
		moon.WithMiddlewares(handlers.HasSession),
		moon.WithApiSpec(specs.GetOneRedirection),
	)
	app.MapGet("/redirections/urls/:id", handlers.GetUrlRedirections,
		moon.WithMiddlewares(handlers.HasSession, redirectionPagination),
		moon.WithApiSpec(specs.GetUrlRedirections),
	)
}

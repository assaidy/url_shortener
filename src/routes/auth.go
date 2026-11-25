package routes

import (
	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/src/handlers"
	"github.com/assaidy/url_shortener/src/specs"
)

func mapAuth(app *moon.App) {
	app.MapPost("/auth/register", handlers.Register,
		moon.WithApiSpec(specs.Register),
	)
	app.MapPost("/auth/login", handlers.Login,
		moon.WithApiSpec(specs.Login),
	)
	app.MapPost("/auth/logout", handlers.Logout,
		moon.WithMiddlewares(handlers.HasSession),
		moon.WithApiSpec(specs.Logout),
	)
	app.MapPut("/auth/usernames", handlers.EditUsername,
		moon.WithMiddlewares(handlers.HasSession, handlers.HasCsrf),
		moon.WithApiSpec(specs.EditUsername),
	)
	app.MapPut("/auth/passwords", handlers.EditPassword,
		moon.WithMiddlewares(handlers.HasSession, handlers.HasCsrf),
		moon.WithApiSpec(specs.EditPassword),
	)
	app.MapGet("/auth/me", handlers.GetAccount,
		moon.WithMiddlewares(handlers.HasSession),
		moon.WithApiSpec(specs.GetAccount),
	)
	app.MapDelete("/auth/me", handlers.DeleteAccount,
		moon.WithMiddlewares(handlers.HasSession, handlers.HasCsrf),
		moon.WithFlags(handlers.FlagDeleteAccount),
		moon.WithApiSpec(specs.DeleteAccount),
	)
}

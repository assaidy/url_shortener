package routes

import "github.com/assaidy/moon"

func Map(app *moon.App) {
	mapDocs(app)
	mapHealth(app)
	mapAuth(app)
	mapShortening(app)
	mapRedirection(app)
}

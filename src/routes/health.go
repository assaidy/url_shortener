package routes

import (
	"github.com/assaidy/moon"
	"github.com/assaidy/moon/handlers/health_check"
	"github.com/assaidy/url_shortener/src/specs"
)

func mapHealth(app *moon.App) {
	app.MapGet(health_check.LivenessEndpoint, health_check.New(),
		moon.WithApiSpec(specs.Liveness),
	)
	app.MapGet(health_check.ReadinessEndpoint, health_check.New(
		health_check.WithProbe(func(ctx *moon.Context) bool {
			return app.CheckServicesReadiness()
		}),
	),
		moon.WithApiSpec(specs.Readiness),
	)
}

package specs

import (
	"net/http"

	"github.com/assaidy/moon"
)

var (
	Liveness = moon.NewApiSpec(
		moon.WithSummary("Liveness probe"),
		moon.WithDescription("Reports whether the application process is alive."),
		moon.WithOperationId("getLiveness"),
		moon.WithTags("Health"),
		moon.WithResponse(emptyResponse(http.StatusOK, "The application is alive.")),
		moon.WithResponse(emptyResponse(http.StatusServiceUnavailable, "The application is not healthy.")),
	)

	Readiness = moon.NewApiSpec(
		moon.WithSummary("Readiness probe"),
		moon.WithDescription("Reports whether the application is ready to serve traffic."),
		moon.WithOperationId("getReadiness"),
		moon.WithTags("Health"),
		moon.WithResponse(emptyResponse(http.StatusOK, "The application is ready to serve traffic.")),
		moon.WithResponse(emptyResponse(http.StatusServiceUnavailable, "The application is not ready to serve traffic.")),
	)
)

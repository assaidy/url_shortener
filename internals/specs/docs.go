package specs

import (
	"net/http"

	"github.com/assaidy/moon"
)

var Swagger = moon.NewApiSpec(
	moon.WithSummary("API documentation"),
	moon.WithDescription("Serves the Swagger UI and the generated OpenAPI document."),
	moon.WithOperationId("getApiDocumentation"),
	moon.WithTags("Documentation"),
	moon.WithResponse(emptyResponse(http.StatusOK, "The Swagger UI or OpenAPI document.")),
)

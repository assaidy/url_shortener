package specs

import (
	"net/http"

	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/internals/handlers"
)

var (
	Redirect = moon.NewApiSpec(
		moon.WithSummary("Resolve a short link"),
		moon.WithDescription("Resolves a public alias and redirects to its destination. Expired or inactive links return 410 Gone."),
		moon.WithOperationId("redirect"),
		moon.WithTags("Redirects"),
		moon.WithParameter(aliasPathParameter()),
		moon.WithResponse(moon.ApiResponse{
			Status:      http.StatusFound,
			Description: "Redirects to the link's destination.",
			Headers: map[string]moon.ApiHeader{
				"Location": {
					Description: "The destination URL.",
					Required:    true,
					Schema:      moon.Object{"type": "string", "format": "uri"},
				},
			},
		}),
		moon.WithResponse(errorResponse(http.StatusNotFound, "The alias was not found.", moon.ErrNotFound)),
		moon.WithResponse(errorResponse(http.StatusGone, "The link is expired or inactive.", moon.ErrGone)),
	)

	GetRedirections = moon.NewApiSpec(
		moon.WithSummary("List redirections"),
		moon.WithDescription("Returns the current user's redirection records ordered by the requested sort key, newest first by default."),
		moon.WithOperationId("getRedirections"),
		moon.WithTags("Analytics"),
		moon.WithSecurity(sessionSecurity),
		moon.WithParameter(limitParameter()),
		moon.WithParameter(cursorParameter()),
		moon.WithParameter(sortParameter("created_at", "os", "platform")),
		moon.WithResponse(jsonResponse(http.StatusOK, "The current user's redirection records.", redirectionPageSchema)),
		moon.WithResponse(errorResponse(http.StatusBadRequest, "The cursor or sort is invalid.", handlers.ErrInvalidCursor, moon.ErrBadRequest)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication failed.", moon.ErrUnauthorized)),
	)

	GetOneRedirection = moon.NewApiSpec(
		moon.WithSummary("Get a redirection"),
		moon.WithDescription("Returns a redirection record owned by the current user."),
		moon.WithOperationId("getOneRedirection"),
		moon.WithTags("Analytics"),
		moon.WithSecurity(sessionSecurity),
		moon.WithParameter(uuidPathParameter("id", "The ID of the redirection.")),
		moon.WithResponse(jsonResponse(http.StatusOK, "The redirection record.", redirectionSchema)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication failed.", moon.ErrUnauthorized)),
		moon.WithResponse(errorResponse(http.StatusNotFound, "The redirection was not found.", handlers.ErrRedirectionNotFound)),
	)

	GetUrlRedirections = moon.NewApiSpec(
		moon.WithSummary("List a link's redirections"),
		moon.WithDescription("Returns the redirection records for a short link owned by the current user, ordered by the requested sort key, newest first by default."),
		moon.WithOperationId("getUrlRedirections"),
		moon.WithTags("Analytics"),
		moon.WithSecurity(sessionSecurity),
		moon.WithParameter(uuidPathParameter("id", "The ID of the short link.")),
		moon.WithParameter(limitParameter()),
		moon.WithParameter(cursorParameter()),
		moon.WithParameter(sortParameter("created_at", "os", "platform")),
		moon.WithResponse(jsonResponse(http.StatusOK, "The link's redirection records.", redirectionPageSchema)),
		moon.WithResponse(errorResponse(http.StatusBadRequest, "The cursor or sort is invalid.", handlers.ErrInvalidCursor, moon.ErrBadRequest)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication failed.", moon.ErrUnauthorized)),
		moon.WithResponse(errorResponse(http.StatusNotFound, "The link was not found.", handlers.ErrUrlNotFound)),
	)
)

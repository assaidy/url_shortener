package specs

import (
	"net/http"

	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/internals/handlers"
)

var (
	CreateUrl = moon.NewApiSpec(
		moon.WithSummary("Create a short link"),
		moon.WithDescription("Creates a short link owned by the current user."),
		moon.WithOperationId("createUrl"),
		moon.WithTags("Links"),
		moon.WithSecurity(sessionCsrfSecurity),
		moon.WithRequestBody(jsonRequestBody("The link details.", createUrlRequestSchema)),
		moon.WithResponse(jsonResponse(http.StatusCreated, "The link was created.", urlSchema)),
		moon.WithResponse(errorResponse(http.StatusBadRequest, "The request body contains invalid JSON.", handlers.ErrInvalidJson)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication or CSRF validation failed.", moon.ErrUnauthorized)),
		moon.WithResponse(errorResponse(http.StatusUnprocessableEntity, "The request payload failed validation.", handlers.ErrInvalidPayload)),
	)

	GetUrls = moon.NewApiSpec(
		moon.WithSummary("List short links"),
		moon.WithDescription("Returns the current user's short links ordered newest first."),
		moon.WithOperationId("getUrls"),
		moon.WithTags("Links"),
		moon.WithSecurity(sessionSecurity),
		moon.WithParameter(limitParameter()),
		moon.WithParameter(cursorParameter()),
		moon.WithResponse(jsonResponse(http.StatusOK, "The current user's short links.", urlPageSchema)),
		moon.WithResponse(errorResponse(http.StatusBadRequest, "The cursor is invalid.", handlers.ErrInvalidCursor)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication failed.", moon.ErrUnauthorized)),
	)

	GetOneUrl = moon.NewApiSpec(
		moon.WithSummary("Get a short link"),
		moon.WithDescription("Returns a short link owned by the current user."),
		moon.WithOperationId("getOneUrl"),
		moon.WithTags("Links"),
		moon.WithSecurity(sessionSecurity),
		moon.WithParameter(uuidPathParameter("id", "The ID of the short link.")),
		moon.WithResponse(jsonResponse(http.StatusOK, "The short link.", urlSchema)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication failed.", moon.ErrUnauthorized)),
		moon.WithResponse(errorResponse(http.StatusNotFound, "The link was not found.", handlers.ErrUrlNotFound)),
	)

	EditUrl = moon.NewApiSpec(
		moon.WithSummary("Edit a short link"),
		moon.WithDescription("Updates the title, expiry, or active status of a short link owned by the current user. The destination, alias, and owner are immutable."),
		moon.WithOperationId("editUrl"),
		moon.WithTags("Links"),
		moon.WithSecurity(sessionCsrfSecurity),
		moon.WithParameter(uuidPathParameter("id", "The ID of the short link.")),
		moon.WithRequestBody(jsonRequestBody("The link fields to update.", editUrlRequestSchema)),
		moon.WithResponse(jsonResponse(http.StatusOK, "The link was updated.", urlSchema)),
		moon.WithResponse(errorResponse(http.StatusBadRequest, "The request body contains invalid JSON.", handlers.ErrInvalidJson)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication or CSRF validation failed.", moon.ErrUnauthorized)),
		moon.WithResponse(errorResponse(http.StatusNotFound, "The link was not found.", handlers.ErrUrlNotFound)),
		moon.WithResponse(errorResponse(http.StatusUnprocessableEntity, "The request payload failed validation.", handlers.ErrInvalidPayload)),
	)

	DeleteUrl = moon.NewApiSpec(
		moon.WithSummary("Delete a short link"),
		moon.WithDescription("Soft-deletes a short link owned by the current user. The alias stays reserved and the link is excluded from management endpoints."),
		moon.WithOperationId("deleteUrl"),
		moon.WithTags("Links"),
		moon.WithSecurity(sessionCsrfSecurity),
		moon.WithParameter(uuidPathParameter("id", "The ID of the short link.")),
		moon.WithResponse(emptyResponse(http.StatusOK, "The link was deleted.")),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication or CSRF validation failed.", moon.ErrUnauthorized)),
		moon.WithResponse(errorResponse(http.StatusNotFound, "The link was not found.", handlers.ErrUrlNotFound)),
	)
)

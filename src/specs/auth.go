package specs

import (
	"net/http"

	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/src/handlers"
)

var (
	Register = moon.NewApiSpec(
		moon.WithSummary("Register an account"),
		moon.WithDescription("Creates a new user account."),
		moon.WithOperationId("register"),
		moon.WithTags("Authentication"),
		moon.WithRequestBody(jsonRequestBody("The account details.", registerRequestSchema)),
		moon.WithResponse(emptyResponse(http.StatusCreated, "The account was created.")),
		moon.WithResponse(errorResponse(http.StatusBadRequest, "The request body contains invalid JSON.", handlers.ErrInvalidJson)),
		moon.WithResponse(errorResponse(http.StatusConflict, "The username is already taken.", handlers.ErrUsernameConflict)),
		moon.WithResponse(errorResponse(http.StatusUnprocessableEntity, "The request payload failed validation.", handlers.ErrInvalidPayload)),
	)

	Login = moon.NewApiSpec(
		moon.WithSummary("Log in"),
		moon.WithDescription("Authenticates a user and sets the session cookies."),
		moon.WithOperationId("login"),
		moon.WithTags("Authentication"),
		moon.WithRequestBody(jsonRequestBody("The login credentials.", loginRequestSchema)),
		moon.WithResponse(emptyResponse(http.StatusOK, "Authentication succeeded. Sets the session_id, session_token, and csrf_token cookies.")),
		moon.WithResponse(errorResponse(http.StatusBadRequest, "The request body contains invalid JSON.", handlers.ErrInvalidJson)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "The credentials are invalid.", moon.ErrUnauthorized)),
		moon.WithResponse(errorResponse(http.StatusUnprocessableEntity, "The request payload failed validation.", handlers.ErrInvalidPayload)),
	)

	Logout = moon.NewApiSpec(
		moon.WithSummary("Log out"),
		moon.WithDescription("Deletes the current session and clears the session cookies."),
		moon.WithOperationId("logout"),
		moon.WithTags("Authentication"),
		moon.WithSecurity(sessionSecurity),
		moon.WithResponse(emptyResponse(http.StatusOK, "The session was deleted.")),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication failed.", moon.ErrUnauthorized)),
	)

	EditUsername = moon.NewApiSpec(
		moon.WithSummary("Change username"),
		moon.WithDescription("Changes the username of the current user."),
		moon.WithOperationId("editUsername"),
		moon.WithTags("Account"),
		moon.WithSecurity(sessionCsrfSecurity),
		moon.WithRequestBody(jsonRequestBody("The new username.", editUsernameRequestSchema)),
		moon.WithResponse(emptyResponse(http.StatusOK, "The username was changed.")),
		moon.WithResponse(errorResponse(http.StatusBadRequest, "The request body contains invalid JSON.", handlers.ErrInvalidJson)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication or CSRF validation failed.", moon.ErrUnauthorized)),
		moon.WithResponse(errorResponse(http.StatusConflict, "The username is already taken.", handlers.ErrUsernameConflict)),
		moon.WithResponse(errorResponse(http.StatusUnprocessableEntity, "The request payload failed validation.", handlers.ErrInvalidPayload)),
	)

	EditPassword = moon.NewApiSpec(
		moon.WithSummary("Change password"),
		moon.WithDescription("Changes the current user's password and signs out all other sessions."),
		moon.WithOperationId("editPassword"),
		moon.WithTags("Account"),
		moon.WithSecurity(sessionCsrfSecurity),
		moon.WithRequestBody(jsonRequestBody("The current and new passwords.", editPasswordRequestSchema)),
		moon.WithResponse(emptyResponse(http.StatusOK, "The password was changed.")),
		moon.WithResponse(errorResponse(http.StatusBadRequest, "The request body contains invalid JSON.", handlers.ErrInvalidJson)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication or CSRF validation failed.", moon.ErrUnauthorized)),
		moon.WithResponse(errorResponse(http.StatusUnprocessableEntity, "The request payload failed validation or the current password is incorrect.", handlers.ErrInvalidPayload, handlers.ErrInvalidOldPassword)),
	)

	GetAccount = moon.NewApiSpec(
		moon.WithSummary("Get account"),
		moon.WithDescription("Returns the current user's account."),
		moon.WithOperationId("getAccount"),
		moon.WithTags("Account"),
		moon.WithSecurity(sessionSecurity),
		moon.WithResponse(jsonResponse(http.StatusOK, "The current user's account.", accountSchema)),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication failed.", moon.ErrUnauthorized)),
	)

	DeleteAccount = moon.NewApiSpec(
		moon.WithSummary("Delete account"),
		moon.WithDescription("Permanently deletes the current user's account and sessions."),
		moon.WithOperationId("deleteAccount"),
		moon.WithTags("Account"),
		moon.WithSecurity(sessionCsrfSecurity),
		moon.WithResponse(emptyResponse(http.StatusOK, "The account was deleted.")),
		moon.WithResponse(errorResponse(http.StatusUnauthorized, "Authentication or CSRF validation failed.", moon.ErrUnauthorized)),
	)
)

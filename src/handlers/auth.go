package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"uuid"

	"github.com/assaidy/moon"
	auth_service "github.com/assaidy/url_shortener/src/services/auth"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	redislock "github.com/jefferyjob/go-redislock"
	redislock_adapter "github.com/jefferyjob/go-redislock/adapter/go-redis/V9"
	"github.com/redis/go-redis/v9"
)

// FlagDeleteAccount marks the route that deletes the current account.
// HasSession skips the shared user lock for this route because DeleteAccount
// takes the exclusive lock itself; taking the shared lock first would make the
// handler wait on its own request.
const FlagDeleteAccount = "delete_account"

// currentSessionLocalKey is the request-local key holding the authenticated
// session set by HasSession.
const currentSessionLocalKey = "handlers.current_session"

var usernameCharsRule = validation.Match(regexp.MustCompile(`^[a-zA-Z0-9_]+$`)).
	Error("may only contain letters, numbers, and underscores")

func Register(ctx *moon.Context) error {
	var request struct {
		Username       string `json:"username"`
		Name           string `json:"name"`
		Password       string `json:"password"`
		VerifyPassword string `json:"verifyPassword"`
	}
	if err := ctx.ReadJson(&request); err != nil {
		return ErrInvalidJson
	}

	request.Name = strings.TrimSpace(request.Name)
	request.Username = strings.TrimSpace(request.Username)

	if err := validation.ValidateStruct(&request,
		validation.Field(&request.Username, validation.Required, validation.Length(1, 32), usernameCharsRule),
		validation.Field(&request.Name, validation.Required, validation.Length(1, 256)),
		validation.Field(&request.Password, validation.Required, validation.Length(8, 50)),
		validation.Field(&request.VerifyPassword, validation.Required, validation.In(request.Password).Error("passwords do not match")),
	); err != nil {
		return ErrInvalidPayload.WithDetails(err)
	}

	authService := ctx.GetDependency[auth_service.Service]()

	if err := authService.Register(ctx, request.Name, request.Username, request.Password); err != nil {
		if errors.Is(err, auth_service.ErrUsernameConflict) {
			return ErrUsernameConflict
		}
		return err
	}

	return ctx.WriteStatus(http.StatusCreated)
}

func Login(ctx *moon.Context) error {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := ctx.ReadJson(&request); err != nil {
		return ErrInvalidJson
	}

	request.Username = strings.TrimSpace(request.Username)

	if err := validation.ValidateStruct(&request,
		validation.Field(&request.Username, validation.Required),
		validation.Field(&request.Password, validation.Required),
	); err != nil {
		return ErrInvalidPayload.WithDetails(err)
	}

	authService := ctx.GetDependency[auth_service.Service]()

	session, err := authService.Login(ctx, request.Username, request.Password, ctx.GetHeader("User-Agent"))
	if err != nil {
		if errors.Is(err, auth_service.ErrUnauthorized) {
			return moon.ErrUnauthorized
		}
		return err
	}

	ctx.SetCookie(&http.Cookie{
		Name:     "session_id",
		Value:    session.Id.String(),
		HttpOnly: true,
		Expires:  session.ExpiresAt,
		Path:     "/",
	})
	ctx.SetCookie(&http.Cookie{
		Name:     "session_token",
		Value:    session.Token,
		HttpOnly: true,
		Expires:  session.ExpiresAt,
		Path:     "/",
	})
	ctx.SetCookie(&http.Cookie{
		Name:     "csrf_token",
		Value:    session.Csrf,
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
		Path:     "/",
	})

	return ctx.WriteStatus(http.StatusOK)
}

func HasSession(ctx *moon.Context) error {
	sessionIdCookie := ctx.GetCookie("session_id")
	sessionToken := ctx.GetCookie("session_token")
	if sessionIdCookie == "" || sessionToken == "" {
		return moon.ErrUnauthorized
	}

	sessionId, err := uuid.Parse(sessionIdCookie)
	if err != nil {
		return moon.ErrUnauthorized
	}

	authService := ctx.GetDependency[auth_service.Service]()

	session, err := authService.GetSession(ctx, sessionId)
	if err != nil {
		if errors.Is(err, auth_service.ErrSessionNotFound) {
			return moon.ErrUnauthorized
		}
		return err
	}

	if session.ExpiresAt.Before(time.Now()) || session.Token != sessionToken {
		return moon.ErrUnauthorized
	}

	// Hold the shared user lock for the rest of the request. Once acquired, the
	// user cannot be deleted until the handler finishes, so services can trust
	// session.UserId without re-checking that the user still exists.
	//
	// The deletion route skips this lock because DeleteAccount takes the
	// exclusive lock itself; otherwise it would wait on its own request.
	if !ctx.IsRouteFlag(FlagDeleteAccount) {
		unlock, err := rlockUser(ctx, session.UserId)
		if err != nil {
			return err
		}
		defer unlock(ctx)

		// The user may have been deleted between the session lookup and
		// acquiring the lock. Re-check under the lock before trusting the
		// session.
		if ok, err := authService.CheckUser(ctx, session.UserId); err != nil {
			return err
		} else if !ok {
			return moon.ErrUnauthorized
		}
	}

	ctx.SetLocal(currentSessionLocalKey, session)
	return ctx.Next()
}

// rlockUser takes the shared user lock for the lifetime of an authenticated
// request. HasSession holds it across the handler chain, so account deletion
// cannot remove the user until the request finishes. The caller must defer the
// returned unlock.
func rlockUser(ctx *moon.Context, userId uuid.UUID) (unlock func(context.Context) error, err error) {
	lock := redislock.New(
		redislock_adapter.New(ctx.GetDependency[*redis.Client]()),
		"users:"+userId.String(),
		redislock.WithAutoRenew(),
	)
	if err := lock.SpinRLock(ctx, 5*time.Second); err != nil {
		return nil, fmt.Errorf("failed to lock user %q: %w", userId, err)
	}
	return lock.RUnLock, nil
}

// wlockUser takes the exclusive user lock for account deletion. It waits for
// in-flight requests holding the shared lock, so they finish before the user is
// deleted. The caller must defer the returned unlock.
func wlockUser(ctx *moon.Context, userId uuid.UUID) (unlock func(context.Context) error, err error) {
	lock := redislock.New(
		redislock_adapter.New(ctx.GetDependency[*redis.Client]()),
		"users:"+userId.String(),
		redislock.WithAutoRenew(),
	)
	if err := lock.SpinWLock(ctx, 5*time.Second); err != nil {
		return nil, fmt.Errorf("failed to lock user %q: %w", userId, err)
	}
	return lock.WUnLock, nil
}

// getCurrentSession returns the session stored by HasSession. It panics when
// the middleware did not run, since that is a route configuration error.
func getCurrentSession(ctx *moon.Context) *auth_service.Session {
	session, ok := ctx.GetLocal[*auth_service.Session](currentSessionLocalKey)
	moon.Assert(ok, "current session not found; HasSession must run before this handler")
	return session
}

func HasCsrf(ctx *moon.Context) error {
	csrfToken := ctx.GetCookie("csrf_token")
	if csrfToken == "" {
		return moon.ErrUnauthorized
	}

	session := getCurrentSession(ctx)

	if session.Csrf != csrfToken {
		return moon.ErrUnauthorized
	}

	return ctx.Next()
}

func Logout(ctx *moon.Context) error {
	session := getCurrentSession(ctx)

	authService := ctx.GetDependency[auth_service.Service]()
	if err := authService.DeleteSession(ctx, session.Id, session.UserId); err != nil {
		if errors.Is(err, auth_service.ErrUserNotFound) || errors.Is(err, auth_service.ErrSessionNotFound) {
			return moon.ErrUnauthorized
		}
		return err
	}

	ctx.ClearCookie("session_id")
	ctx.ClearCookie("session_token")
	ctx.ClearCookie("csrf_token")

	return ctx.WriteStatus(http.StatusOK)
}

func EditUsername(ctx *moon.Context) error {
	var request struct {
		Username string `json:"username"`
	}
	if err := ctx.ReadJson(&request); err != nil {
		return ErrInvalidJson
	}

	request.Username = strings.TrimSpace(request.Username)

	if err := validation.ValidateStruct(&request,
		validation.Field(&request.Username, validation.Required, validation.Length(1, 32), usernameCharsRule),
	); err != nil {
		return ErrInvalidPayload.WithDetails(err)
	}

	session := getCurrentSession(ctx)

	authService := ctx.GetDependency[auth_service.Service]()

	if err := authService.EditUsername(ctx, session.UserId, request.Username); err != nil {
		if errors.Is(err, auth_service.ErrUsernameConflict) {
			return ErrUsernameConflict
		}
		if errors.Is(err, auth_service.ErrUserNotFound) {
			return moon.ErrUnauthorized
		}
		return err
	}

	return ctx.WriteStatus(http.StatusOK)
}

func EditPassword(ctx *moon.Context) error {
	var request struct {
		OldPassword       string `json:"oldPassword"`
		NewPassword       string `json:"newPassword"`
		VerifyNewPassword string `json:"verifyNewPassword"`
	}
	if err := ctx.ReadJson(&request); err != nil {
		return ErrInvalidJson
	}

	if err := validation.ValidateStruct(&request,
		validation.Field(&request.OldPassword, validation.Required),
		validation.Field(&request.NewPassword, validation.Required, validation.Length(8, 50)),
		validation.Field(&request.VerifyNewPassword, validation.Required, validation.In(request.NewPassword).Error("passwords do not match")),
	); err != nil {
		return ErrInvalidPayload.WithDetails(err)
	}

	session := getCurrentSession(ctx)

	authService := ctx.GetDependency[auth_service.Service]()

	if err := authService.EditPassword(ctx, session.Id, session.UserId, request.OldPassword, request.NewPassword); err != nil {
		if errors.Is(err, auth_service.ErrInvalidOldPassword) {
			return ErrInvalidOldPassword
		}
		if errors.Is(err, auth_service.ErrUserNotFound) {
			return moon.ErrUnauthorized
		}
		return err
	}

	return ctx.WriteStatus(http.StatusOK)
}

func GetAccount(ctx *moon.Context) error {
	session := getCurrentSession(ctx)

	authService := ctx.GetDependency[auth_service.Service]()

	account, err := authService.GetAccount(ctx, session.UserId)
	if err != nil {
		if errors.Is(err, auth_service.ErrUserNotFound) {
			return moon.ErrUnauthorized
		}
		return err
	}

	return ctx.WriteJson(http.StatusOK, moon.Object{
		"id":        account.Id,
		"name":      account.Name,
		"username":  account.Username,
		"createdAt": account.CreatedAt,
	})
}

func DeleteAccount(ctx *moon.Context) error {
	session := getCurrentSession(ctx)

	// HasSession skipped the shared user lock for this route. Take the
	// exclusive lock so in-flight authenticated requests finish before the user
	// and their sessions are deleted.
	unlock, err := wlockUser(ctx, session.UserId)
	if err != nil {
		return err
	}
	defer unlock(ctx)

	authService := ctx.GetDependency[auth_service.Service]()

	if err := authService.DeleteAccount(ctx, session.UserId); err != nil {
		if errors.Is(err, auth_service.ErrUserNotFound) {
			return moon.ErrUnauthorized
		}
		return err
	}

	ctx.ClearCookie("session_id")
	ctx.ClearCookie("session_token")
	ctx.ClearCookie("csrf_token")

	return ctx.WriteStatus(http.StatusOK)
}

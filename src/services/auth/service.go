// Package auth provides account, credential, and session management backed by
// PostgreSQL.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/src/events"
	"github.com/assaidy/url_shortener/src/services/auth/queries"
	postgres_service "github.com/assaidy/url_shortener/src/services/postgres"
	redis_service "github.com/assaidy/url_shortener/src/services/redis"
	"github.com/assaidy/url_shortener/src/utils"
	"github.com/assaidy/workers"
	"github.com/assaidy/workers/lock"
	redislock "github.com/jefferyjob/go-redislock"
	redislock_adapter "github.com/jefferyjob/go-redislock/adapter/go-redis/V9"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var _ moon.Service = (*Service)(nil)

// Service manages user accounts, credentials, and authentication sessions.
type Service struct {
	db            *sql.DB
	queries       *queries.Queries
	rdb           *redis.Client
	logger        *slog.Logger
	workerManager *workers.WorkerManager
}

// New returns a Service backed by db.
func New() moon.ServiceConstructor[*Service] {
	return func(i moon.Injected) *Service {
		logger := i.GetDependency[*slog.Logger]()
		db := i.GetService[*postgres_service.Service]().GetDb()
		rdb := i.GetService[*redis_service.Service]().GetClient()

		service := &Service{
			db:      db,
			queries: queries.New(db),
			rdb:     rdb,
			logger:  logger.WithGroup("auth"),
			workerManager: workers.NewWorkerManager(
				workers.WithLogger(logger.WithGroup("auth:workers")),
				workers.WithLockGenerator(lock.NewRedisGenerator(rdb)),
			),
		}

		service.workerManager.RegisterWorker(workers.NewWorker(
			"session cleanup",
			service.sessionCleanupJob,
			workers.WithSchedules(workers.WeeklyAt(time.Friday, 2, 0)),
			workers.WithTimeout(10*time.Minute),
			workers.WithBackoffStrategy(workers.DecorrelatedJitterBackoff(10*time.Minute)),
			workers.WithSingleInstance(),
		))

		return service
	}
}

// Name implements [moon.Service].
func (me *Service) Name() string {
	return "auth"
}

// Start implements [moon.Service].
func (me *Service) Start() error {
	me.workerManager.Start()
	return nil
}

// Stop implements [moon.Service].
func (me *Service) Stop() error {
	me.workerManager.Stop()
	return nil
}

// IsReady implements [moon.Service].
func (me *Service) IsReady() bool {
	return true
}

// Register creates a user account with the given name, username, and password.
// It returns ErrUsernameConflict if the username is already taken.
func (me *Service) Register(ctx context.Context, name, username, password string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password")
	}

	unlock, err := me.lockUsername(ctx, username)
	if err != nil {
		return err
	}
	defer unlock(ctx)

	if n, err := me.queries.InsertUser(ctx, queries.InsertUserParams{
		Id:           uuid.NewV7(),
		Name:         name,
		Username:     username,
		PasswordHash: string(passwordHash),
	}); err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	} else if n == 0 {
		return ErrUsernameConflict
	}

	return nil
}

type unlockFunc func(ctx context.Context) error

func (me *Service) lockUsername(ctx context.Context, username string) (unlockFunc, error) {
	lock := redislock.New(redislock_adapter.New(me.rdb), "username:"+username, redislock.WithAutoRenew())
	if err := lock.SpinLock(ctx, 5*time.Second); err != nil {
		return nil, fmt.Errorf("failed to lock username %q: %w", username, err)
	}
	return lock.UnLock, nil
}

// Session represents an authenticated session and the client device that
// created it.
type Session struct {
	Id        uuid.UUID
	UserId    uuid.UUID
	Token     string
	Csrf      string
	ExpiresAt time.Time
	CreatedAt time.Time
	Platform  string
	Os        string
}

// Login verifies the supplied credentials and creates a session. The raw
// User-Agent is used to record the session's platform and operating system. It
// returns ErrUnauthorized when the credentials are invalid.
func (me *Service) Login(ctx context.Context, username, password string, userAgent string) (*Session, error) {
	platform := utils.ExtractPlatformFromUserAgent(userAgent)
	os := utils.ExtractOsFromUserAgent(userAgent)

	tx, err := me.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := me.queries.WithTx(tx)

	user, err := qtx.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, ErrUnauthorized
	}

	sessionId := uuid.NewV7()
	token := moon.GenerateSecureToken()
	csrf := moon.GenerateSecureToken()
	expirationTime := time.Now().Add(120 * 24 * time.Hour)

	if err := qtx.InsertSession(ctx, queries.InsertSessionParams{
		Id:        sessionId,
		UserId:    user.Id,
		Token:     token,
		Csrf:      csrf,
		ExpiresAt: expirationTime,
		Platform:  platform,
		Os:        os,
	}); err != nil {
		return nil, fmt.Errorf("failed to insert session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed ot commit tx: %w", err)
	}

	return &Session{
		Id:        sessionId,
		UserId:    user.Id,
		Token:     token,
		Csrf:      csrf,
		ExpiresAt: expirationTime,
		CreatedAt: time.Now(),
		Platform:  platform,
		Os:        os,
	}, nil
}

// GetSession returns the session with the given ID. It returns
// ErrSessionNotFound when no such session exists.
func (me *Service) GetSession(ctx context.Context, sessionId uuid.UUID) (*Session, error) {
	session, err := me.queries.GetSessionById(ctx, sessionId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session by id: %w", err)
	}

	return &Session{
		Id:        session.Id,
		UserId:    session.UserId,
		Token:     session.Token,
		Csrf:      session.Csrf,
		ExpiresAt: session.ExpiresAt,
		CreatedAt: session.CreatedAt,
		Platform:  session.Platform,
		Os:        session.Os,
	}, nil
}

// DeleteSession deletes a session owned by userId. It returns ErrUserNotFound
// when the user does not exist and ErrSessionNotFound when the user owns no
// session with the given ID.
func (me *Service) DeleteSession(ctx context.Context, sessionId, userId uuid.UUID) error {
	tx, err := me.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := me.queries.WithTx(tx)

	if ok, err := qtx.CheckUserId(ctx, userId); err != nil {
		return fmt.Errorf("failed to check user id: %w", err)
	} else if !ok {
		return ErrUserNotFound
	}

	if n, err := qtx.DeleteUserSession(ctx, queries.DeleteUserSessionParams{
		Id:     sessionId,
		UserId: userId,
	}); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	} else if n == 0 {
		return ErrSessionNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed ot commit tx: %w", err)
	}

	return nil
}

// EditUsername changes the username of the given user. It returns
// ErrUserNotFound when the user does not exist and ErrUsernameConflict when the
// new username is already taken.
func (me *Service) EditUsername(ctx context.Context, userId uuid.UUID, newUsername string) error {
	unlock, err := me.lockUsername(ctx, newUsername)
	if err != nil {
		return err
	}
	defer unlock(ctx)

	result, err := me.queries.UpdateUsername(ctx, queries.UpdateUsernameParams{
		Id:       userId,
		Username: newUsername,
	})
	if err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}

	if !result.UserExists {
		return ErrUserNotFound
	}
	if !result.Updated {
		return ErrUsernameConflict
	}

	return nil
}

// TODO: Reset a forgotten password using a emailed token.

// EditPassword changes the user's password after verifying the current one.
// All of the user's other sessions are deleted, keeping sessionId active. It
// returns ErrUserNotFound or ErrInvalidOldPassword when appropriate.
func (me *Service) EditPassword(ctx context.Context, sessionId, userId uuid.UUID, oldPassword, newPassword string) error {
	tx, err := me.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := me.queries.WithTx(tx)

	user, err := qtx.GetUserById(ctx, userId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user by id: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)) != nil {
		return ErrInvalidOldPassword
	}

	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password")
	}

	if err := qtx.UpdatePasswordHash(ctx, queries.UpdatePasswordHashParams{
		Id:           userId,
		PasswordHash: string(newPasswordHash),
	}); err != nil {
		return fmt.Errorf("failed to update password hash: %w", err)
	}

	if err := qtx.DeleteAllUserSessionsExceptCurrent(ctx, queries.DeleteAllUserSessionsExceptCurrentParams{
		UserId:  userId,
		Current: sessionId,
	}); err != nil {
		return fmt.Errorf("failed to delete all user sessions except current: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed ot commit tx: %w", err)
	}

	return nil
}

// Account is the public profile of a user.
type Account struct {
	Id        uuid.UUID
	Name      string
	Username  string
	CreatedAt time.Time
}

// GetAccount returns the public profile of the given user. It returns
// ErrUserNotFound when the user does not exist.
func (me *Service) GetAccount(ctx context.Context, userId uuid.UUID) (*Account, error) {
	user, err := me.queries.GetUserById(ctx, userId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &Account{
		Id:        user.Id,
		Name:      user.Name,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	}, nil
}

// DeleteAccount soft-deletes the given user and deletes their sessions. It
// returns ErrUserNotFound when the user does not exist.
func (me *Service) DeleteAccount(ctx context.Context, userId uuid.UUID) error {
	tx, err := me.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := me.queries.WithTx(tx)

	if n, err := qtx.SoftDeleteUser(ctx, userId); err != nil {
		return fmt.Errorf("failed to soft delete user: %w", err)
	} else if n == 0 {
		return ErrUserNotFound
	}

	if err := qtx.DeleteAllUserSessions(ctx, userId); err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	// The account is already deleted, so a failed publish must not fail the
	// request. Publish after commit so consumers never see an event for a
	// deletion that was rolled back.
	publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := events.Enqueu(
		publishCtx,
		me.rdb,
		events.UserDeletedEvent,
		events.UserDeletedEventPayload{UserId: userId},
	); err != nil {
		me.logger.Error("failed to publish user deleted event", "user_id", userId, "error", err)
	}

	return nil
}

// CheckUser reports whether userId belongs to a user that exists and has not been soft-deleted.
func (me *Service) CheckUser(ctx context.Context, userId uuid.UUID) (bool, error) {
	ok, err := me.queries.CheckUserId(ctx, userId)
	if err != nil {
		return false, fmt.Errorf("failed to check user id: %w", err)
	}
	return ok, nil
}

func (me *Service) sessionCleanupJob(ctx context.Context) error {
	if err := me.queries.BatchDeleteExpiredOrSoftDeletedSessions(ctx); err != nil {
		return fmt.Errorf("failed to batch delete expired or soft-deleted sessions: %w", err)
	}

	return nil
}

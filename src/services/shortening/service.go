// Package shortening provides short url creation, management, and lookup
// backed by PostgreSQL.
package shortening

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"
	"uuid"

	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/src/events"
	"github.com/assaidy/url_shortener/src/services/shortening/queries"
	"github.com/assaidy/url_shortener/src/utils"
	"github.com/assaidy/workers"
	"github.com/assaidy/workers/lock"
	"github.com/redis/go-redis/v9"
)

const urlClickEventConsumers = 4

// Service manages short urls: creating, listing, updating, deleting, and
// resolving them by alias.
type Service interface {
	// CreateUrl creates a short url owned by userId and returns it. The alias
	// is generated randomly and its length is grown as needed until it is
	// unique. A zero expiresAt means the url never expires.
	//
	// It returns no sentinel errors.
	CreateUrl(ctx context.Context, userId uuid.UUID, title, url string, expiresAt time.Time) (*Url, error)

	// GetUserUrls returns a page of the non-deleted urls owned by userId,
	// filtered by search and ordered by sort. The last* arguments are the
	// cursor: when lastId is the nil UUID the first page is returned, and
	// limit caps the page size.
	//
	// Errors:
	//   - [ErrInvalidSort]: sort is not a known [UserUrlsSort] value.
	GetUserUrls(
		ctx context.Context,
		userId uuid.UUID,
		search string,
		sort UserUrlsSort,
		lastTitle, lastUrl, lastAlias string, lastCreatedAt time.Time, lastId uuid.UUID,
		limit int,
	) ([]Url, error)

	// GetUserUrl returns the url with the given ID owned by userId.
	//
	// Errors:
	//   - [ErrUrlNotFound]: the user owns no such url.
	GetUserUrl(ctx context.Context, userId, urlId uuid.UUID) (*Url, error)

	// UpdateUserUrl updates the title, expiration, and active state of the url
	// with the given ID owned by userId, and returns the updated url.
	//
	// Errors:
	//   - [ErrUrlNotFound]: the user owns no such url.
	UpdateUserUrl(ctx context.Context, userId, urlId uuid.UUID, title string, expiresAt time.Time, isActive bool) (*Url, error)

	// DeleteUserUrl soft-deletes the url with the given ID owned by userId.
	//
	// Errors:
	//   - [ErrUrlNotFound]: the user owns no such non-deleted url.
	DeleteUserUrl(ctx context.Context, userId, urlId uuid.UUID) error

	// GetUrlByAlias returns the non-deleted url with the given alias.
	//
	// Errors:
	//   - [ErrUrlNotFound]: no such url exists.
	GetUrlByAlias(ctx context.Context, alias string) (*Url, error)
}

var (
	_ moon.Service = (*impl)(nil)
	_ Service      = (*impl)(nil)
)

// impl is the PostgreSQL and Redis backed implementation of [Service].
type impl struct {
	db            *sql.DB
	queries       *queries.Queries
	rdb           *redis.Client
	workerManager *workers.WorkerManager
}

// New returns a Service backed by the database and Redis client found in the
// dependency container.
func New() moon.ServiceConstructor {
	return func(deps *moon.DependencyContainer) moon.Service {
		logger := deps.Get[*slog.Logger]()
		db := deps.Get[*sql.DB]()
		rdb := deps.Get[*redis.Client]()

		service := &impl{
			db:      db,
			queries: queries.New(db),
			rdb:     rdb,
			workerManager: workers.NewWorkerManager(
				workers.WithLogger(logger.WithGroup("shortening:workers")),
				workers.WithLockGenerator(lock.NewRedisGenerator(rdb)),
			),
		}

		service.workerManager.RegisterWorker(
			workers.NewWorker(
				fmt.Sprintf("%q event consumer", events.UserDeletedEvent),
				service.userDeletedEventConsumerJob,
				workers.WithRetryDelay(time.Second),
				workers.WithBackoffStrategy(workers.DecorrelatedJitterBackoff(10*time.Minute)),
			),
		)

		for i := range urlClickEventConsumers {
			service.workerManager.RegisterWorker(
				workers.NewWorker(
					fmt.Sprintf("%q event consumer #%d", events.UrlClickedEvent, i+1),
					service.urlClickedEventConsumerJob,
					workers.WithRetryDelay(time.Second),
					workers.WithBackoffStrategy(workers.DecorrelatedJitterBackoff(10*time.Minute)),
				),
			)
		}

		return service
	}
}

// Name implements [moon.Service].
func (me *impl) Name() string {
	return "urls"
}

// Start implements [moon.Service].
func (me *impl) Start(deps *moon.DependencyContainer) error {
	me.workerManager.Start()
	deps.Set(Service(me))
	return nil
}

// Stop implements [moon.Service].
func (me *impl) Stop(deps *moon.DependencyContainer) error {
	deps.Delete[Service]()
	me.workerManager.Stop()
	return nil
}

// IsReady implements [moon.Service].
func (me *impl) IsReady() bool {
	return true
}

// Url is a short url and its metadata.
type Url struct {
	Id        uuid.UUID
	UserId    uuid.UUID
	Title     string
	Url       string
	Alias     string
	CreatedAt time.Time
	ExpiresAt *time.Time
	IsActive  bool
	Clicks    int64
}

func (me *impl) CreateUrl(ctx context.Context, userId uuid.UUID, title, url string, expiresAt time.Time) (*Url, error) {
	length, err := me.queries.GetAliasLength(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get alias length: %w", err)
	}

	id := uuid.NewV7()
	createdAt := time.Now()
	var alias string

	var expiresAtPtr *time.Time
	if !expiresAt.IsZero() {
		expiresAtPtr = &expiresAt
	}

	for {
		for range 5 {
			alias = generateRandomLetters(int(length))
			if n, err := me.queries.InsertUrl(ctx, queries.InsertUrlParams{
				Id:        id,
				UserId:    userId,
				Title:     title,
				Url:       url,
				Alias:     alias,
				ExpiresAt: utils.NullTime(expiresAt),
				CreatedAt: createdAt,
			}); err != nil {
				return nil, fmt.Errorf("failed to insert url: %w", err)
			} else if n != 0 {
				return &Url{
					Id:        id,
					UserId:    userId,
					Title:     title,
					Url:       url,
					Alias:     alias,
					CreatedAt: createdAt,
					ExpiresAt: expiresAtPtr,
					IsActive:  true,
				}, nil
			}
		}

		// TODO: cap the alias length so a bug that keeps inserts from succeeding
		// can't make this loop grow the alias length forever.
		length, err = me.queries.IncrementAliasLength(ctx, length)
		if err != nil {
			return nil, fmt.Errorf("failed to increment alias length: %w", err)
		}
	}
}

func generateRandomLetters(length int) string {
	const allLetters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	buffer := make([]byte, 0, length)
	for range length {
		buffer = append(buffer, allLetters[rand.IntN(len(allLetters))])
	}
	return string(buffer)
}

// UserUrlsSort is a sort order for [Service.GetUserUrls].
type UserUrlsSort string

const (
	SortTitleAscending      UserUrlsSort = "title.asc"
	SortTitleDescending     UserUrlsSort = "title.desc"
	SortUrlAscending        UserUrlsSort = "url.asc"
	SortUrlDescending       UserUrlsSort = "url.desc"
	SortAliasAscending      UserUrlsSort = "alias.asc"
	SortAliasDescending     UserUrlsSort = "alias.desc"
	SortCreatedAtAscending  UserUrlsSort = "created_at.asc"
	SortCreatedAtDescending UserUrlsSort = "created_at.desc"
)

func (me UserUrlsSort) isValid() bool {
	switch me {
	case SortCreatedAtAscending, SortCreatedAtDescending,
		SortTitleAscending, SortTitleDescending,
		SortUrlAscending, SortUrlDescending,
		SortAliasAscending, SortAliasDescending:
		return true
	}
	return false
}

func (me *impl) GetUserUrls(
	ctx context.Context,
	userId uuid.UUID,
	search string,
	sort UserUrlsSort,
	lastTitle, lastUrl, lastAlias string, lastCreatedAt time.Time, lastId uuid.UUID,
	limit int,
) ([]Url, error) {
	if !sort.isValid() {
		return nil, ErrInvalidSort
	}

	// TODO: use FTS using title, url, and alias (see the query)
	rows, err := me.queries.GetUserUrls(ctx, queries.GetUserUrlsParams{
		UserId:        userId,
		Search:        search,
		LastTitle:     utils.NullString(lastTitle),
		LastUrl:       utils.NullString(lastUrl),
		LastAlias:     utils.NullString(lastAlias),
		LastCreatedAt: utils.NullTime(lastCreatedAt),
		LastId:        utils.UuidPtr(lastId),
		Sort:          string(sort),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user urls: %w", err)
	}

	urls := make([]Url, 0, len(rows))
	for _, row := range rows {
		urls = append(urls, toUrl(row))
	}

	return urls, nil
}

func (me *impl) GetUserUrl(ctx context.Context, userId, urlId uuid.UUID) (*Url, error) {
	row, err := me.queries.GetUserUrlById(ctx, queries.GetUserUrlByIdParams{
		Id:     urlId,
		UserId: userId,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUrlNotFound
		}
		return nil, fmt.Errorf("failed to get user url: %w", err)
	}

	return new(toUrl(row)), nil
}

func (me *impl) UpdateUserUrl(ctx context.Context, userId, urlId uuid.UUID, title string, expiresAt time.Time, isActive bool) (*Url, error) {
	row, err := me.queries.UpdateUserUrl(ctx, queries.UpdateUserUrlParams{
		Id:        urlId,
		UserId:    userId,
		Title:     title,
		ExpiresAt: utils.NullTime(expiresAt),
		IsActive:  isActive,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUrlNotFound
		}
		return nil, fmt.Errorf("failed to update user url: %w", err)
	}

	return new(toUrl(row)), nil
}

func (me *impl) DeleteUserUrl(ctx context.Context, userId, urlId uuid.UUID) error {
	if n, err := me.queries.SoftDeleteUserUrl(ctx, queries.SoftDeleteUserUrlParams{
		Id:     urlId,
		UserId: userId,
	}); err != nil {
		return fmt.Errorf("failed to delete user url: %w", err)
	} else if n == 0 {
		return ErrUrlNotFound
	}

	return nil
}

func (me *impl) GetUrlByAlias(ctx context.Context, alias string) (*Url, error) {
	row, err := me.queries.GetUrlByAlias(ctx, alias)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUrlNotFound
		}
		return nil, fmt.Errorf("failed to get url by alias: %w", err)
	}

	return new(toUrl(row)), nil
}

func (me *impl) userDeletedEventConsumerJob(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			var payload events.UserDeletedEventPayload
			if ok, err := events.Dequeue(ctx, me.rdb, events.UserDeletedEvent, &payload); err != nil {
				return fmt.Errorf("failed to dequeue %q event: %w", events.UserDeletedEvent, err)
			} else if !ok {
				continue
			}

			if _, err := me.queries.SoftDeleteUserUrls(ctx, payload.UserId); err != nil {
				return fmt.Errorf("failed to soft-delete user urls: %w", err)
			}
		}
	}
}

func (me *impl) urlClickedEventConsumerJob(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			var payload events.UrlClickedEventPayload
			if ok, err := events.Dequeue(ctx, me.rdb, events.UrlClickedEvent, &payload); err != nil {
				return fmt.Errorf("failed to dequeue %q event: %w", events.UrlClickedEvent, err)
			} else if !ok {
				continue
			}

			if err := me.queries.IncrementUrlClicks(ctx, payload.UrlId); err != nil {
				return fmt.Errorf("failed to increment url clicks: %w", err)
			}
		}
	}
}

func toUrl(r queries.ShorteningUrl) Url {
	u := Url{
		Id:        r.Id,
		UserId:    r.UserId,
		Title:     r.Title,
		Url:       r.Url,
		Alias:     r.Alias,
		CreatedAt: r.CreatedAt,
		IsActive:  r.IsActive,
		Clicks:    r.Clicks,
	}
	if r.ExpiresAt.Valid {
		u.ExpiresAt = &r.ExpiresAt.Time
	}
	return u
}

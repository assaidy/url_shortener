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
	"github.com/assaidy/url_shortener/internals/events"
	postgres_service "github.com/assaidy/url_shortener/internals/services/postgres"
	redis_service "github.com/assaidy/url_shortener/internals/services/redis"
	"github.com/assaidy/url_shortener/internals/services/shortening/queries"
	"github.com/assaidy/url_shortener/internals/utils"
	"github.com/assaidy/workers"
	"github.com/assaidy/workers/lock"
	"github.com/redis/go-redis/v9"
)

const urlClickEventConsumers = 4

var _ moon.Service = (*Service)(nil)

type Service struct {
	db            *sql.DB
	queries       *queries.Queries
	rdb           *redis.Client
	workerManager *workers.WorkerManager
}

// New returns a shortening service backed by db.
func New() moon.ServiceConstructor[*Service] {
	return func(i moon.Injected) *Service {
		logger := i.GetDependency[*slog.Logger]()
		db := i.GetService[*postgres_service.Service]().GetDb()
		rdb := i.GetService[*redis_service.Service]().GetClient()

		service := &Service{
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
func (s *Service) Name() string {
	return "urls"
}

// Start implements [moon.Service].
func (s *Service) Start() error {
	s.workerManager.Start()
	return nil
}

// Stop implements [moon.Service].
func (s *Service) Stop() error {
	s.workerManager.Stop()
	return nil
}

// IsReady implements [moon.Service].
func (s *Service) IsReady() bool {
	return true
}

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

// CreateUrl creates a short url owned by userId and returns it. The alias is
// generated and grown as needed until it is unique.
func (me *Service) CreateUrl(ctx context.Context, userId uuid.UUID, title, url string, expiresAt time.Time) (*Url, error) {
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

// GetUserUrls returns the urls owned by userId ordered newest first. When
// lastId is the nil UUID the first page is returned; limit caps the page size.
func (me *Service) GetUserUrls(
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

// GetUserUrl returns the url with the given ID owned by userId. It returns
// ErrUrlNotFound when the user owns no such url.
func (me *Service) GetUserUrl(ctx context.Context, userId, urlId uuid.UUID) (*Url, error) {
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

// UpdateUserUrl applies update to the url with the given ID owned by userId. It
// returns ErrUrlNotFound when the user owns no such url.
func (me *Service) UpdateUserUrl(ctx context.Context, userId, urlId uuid.UUID, title string, expiresAt time.Time, isActive bool) (*Url, error) {
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

// DeleteUserUrl soft-deletes the url with the given ID owned by userId. It
// returns ErrUrlNotFound when the user owns no such non-deleted url.
func (me *Service) DeleteUserUrl(ctx context.Context, userId, urlId uuid.UUID) error {
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

func (me *Service) userDeletedEventConsumerJob(ctx context.Context) error {
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

func (me *Service) urlClickedEventConsumerJob(ctx context.Context) error {
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

// GetUrlByAlias returns the non-deleted url with the given alias. It returns
// ErrUrlNotFound when no such url exists.
func (me *Service) GetUrlByAlias(ctx context.Context, alias string) (*Url, error) {
	row, err := me.queries.GetUrlByAlias(ctx, alias)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUrlNotFound
		}
		return nil, fmt.Errorf("failed to get url by alias: %w", err)
	}

	return new(toUrl(row)), nil
}

// Package redirection records clicks on public short links and exposes the
// recorded redirections.
package redirection

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"uuid"

	"github.com/assaidy/moon"
	"github.com/assaidy/url_shortener/src/events"
	"github.com/assaidy/url_shortener/src/services/redirection/queries"
	"github.com/assaidy/url_shortener/src/utils"
	"github.com/assaidy/workers"
	"github.com/redis/go-redis/v9"
)

const (
	redirectionBatchSize      = 500
	redirectionFlushInterval  = time.Second
	redirectionIngressSize    = 1024
	redirectionBatchQueueSize = 4
	redirectionFlushTimeout   = 10 * time.Second
)

// Service records redirections (clicks on short links) and lets callers read
// them back.
type Service interface {
	// Record buffers a click on the url urlId, owned by userId, for
	// asynchronous persistence. Records are written in batches by a background
	// flusher, so a nil error only means the click was accepted for recording;
	// it does not mean it has been written to the database.
	//
	// It returns no sentinel errors. If ctx ends while the buffer is full, the
	// wrapped context error is returned.
	Record(ctx context.Context, urlId, userId uuid.UUID, ip, platform, os string) error

	// GetUserRedirections returns a page of the redirections of all urls owned
	// by userId, ordered by sort. The last* arguments are the cursor: when
	// lastRedirectionId is the nil UUID the first page is returned, and limit
	// caps the page size.
	//
	// Errors:
	//   - [ErrInvalidSort]: sort is not a known [RedirectionsSort] value.
	GetUserRedirections(
		ctx context.Context,
		userId uuid.UUID,
		sort RedirectionsSort,
		lastCreatedAt time.Time, lastOs, lastPlatform string, lastRedirectionId uuid.UUID,
		limit int,
	) ([]Redirection, error)

	// GetRedirectionById returns the redirection with the given ID that
	// belongs to userId.
	//
	// Errors:
	//   - [ErrRedirectionNotFound]: the user has no redirection with this ID.
	GetRedirectionById(ctx context.Context, redirectionId, userId uuid.UUID) (*Redirection, error)

	// GetUrlRedirections returns a page of the redirections of the url urlId,
	// ordered by sort. The last* arguments are the cursor: when
	// lastRedirectionId is the nil UUID the first page is returned, and limit
	// caps the page size.
	//
	// Redirections are a historical record and are kept after their url is
	// deleted, so this does not check that the url exists or who owns it.
	// Callers must authorize access to urlId first.
	//
	// Errors:
	//   - [ErrInvalidSort]: sort is not a known [RedirectionsSort] value.
	GetUrlRedirections(
		ctx context.Context,
		urlId uuid.UUID,
		sort RedirectionsSort,
		lastCreatedAt time.Time, lastOs, lastPlatform string, lastRedirectionId uuid.UUID,
		limit int,
	) ([]Redirection, error)
}

var (
	_ Service      = (*impl)(nil)
	_ moon.Service = (*impl)(nil)
)

// impl is the PostgreSQL and Redis backed implementation of [Service].
type impl struct {
	db                      *sql.DB
	queries                 *queries.Queries
	rdb                     *redis.Client
	logger                  *slog.Logger
	workerManager           *workers.WorkerManager
	redirectionRecords      chan redirectionRecord
	redirectionBatches      chan *[]redirectionRecord
	redirectionPool         sync.Pool
	currentRedirectionBatch *[]redirectionRecord
	lastRedirectionFlush    time.Time
}

// New returns a Service backed by the database and Redis client found in the
// dependency container.
func New() moon.ServiceConstructor {
	return func(deps *moon.DependencyContainer) moon.Service {
		logger := deps.Get[*slog.Logger]()
		db := deps.Get[*sql.DB]()
		rdb := deps.Get[*redis.Client]()

		service := &impl{
			db:                 db,
			queries:            queries.New(db),
			rdb:                rdb,
			logger:             logger.WithGroup("redirection"),
			workerManager:      workers.NewWorkerManager(workers.WithLogger(logger.WithGroup("redirection:workers"))),
			redirectionRecords: make(chan redirectionRecord, redirectionIngressSize),
			redirectionBatches: make(chan *[]redirectionRecord, redirectionBatchQueueSize),
			redirectionPool: sync.Pool{New: func() any {
				return new(make([]redirectionRecord, 0, redirectionBatchSize))
			}},
			currentRedirectionBatch: new(make([]redirectionRecord, 0, redirectionBatchSize)),
		}

		service.workerManager.RegisterWorker(workers.NewWorker(
			"redirection records buffer",
			service.redirectionRecordsBufferJob,
		))
		service.workerManager.RegisterWorker(workers.NewWorker(
			"redirection records flusher",
			service.redirectionRecordsFlusherJob,
		))

		return service
	}
}

// Name implements [moon.Service].
func (me *impl) Name() string {
	return "redirection"
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
	close(me.redirectionRecords)
	me.workerManager.Stop()
	return nil
}

// IsReady implements [moon.Service].
func (me *impl) IsReady() bool {
	return true
}

type redirectionRecord struct {
	Id        uuid.UUID `json:"id"`
	UrlId     uuid.UUID `json:"url_id"`
	UserId    uuid.UUID `json:"user_id"`
	Ip        string    `json:"ip"`
	Platform  string    `json:"platform"`
	Os        string    `json:"os"`
	CreatedAt time.Time `json:"created_at"`
}

func (me *impl) flushRedirectionBatch(batch []redirectionRecord) {
	raw, err := json.Marshal(batch)
	if err != nil {
		me.logger.Error("failed to marshal redirection batch", "count", len(batch), "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), redirectionFlushTimeout)
	defer cancel()
	if err := me.queries.InsertRedirections(ctx, raw); err != nil {
		me.logger.Error("failed to insert redirection batch", "count", len(batch), "error", err)
	}

	for _, record := range batch {
		if err := events.Enqueu(
			ctx,
			me.rdb,
			events.UrlClickedEvent,
			events.UrlClickedEventPayload{UrlId: record.UrlId},
		); err != nil {
			me.logger.Error("failed to publish url clicked event", "url_id", record.UrlId, "error", err)
		}
	}
}

func (me *impl) recycleRedirectionBatch(batch *[]redirectionRecord) {
	clear(*batch)
	*batch = (*batch)[:0]
	me.redirectionPool.Put(batch)
}

func (me *impl) addToRedirectionBatch(record redirectionRecord) {
	*me.currentRedirectionBatch = append(*me.currentRedirectionBatch, record)
	if len(*me.currentRedirectionBatch) == redirectionBatchSize {
		me.redirectionBatches <- me.currentRedirectionBatch
		me.currentRedirectionBatch = me.redirectionPool.Get().(*[]redirectionRecord)
		me.lastRedirectionFlush = time.Now()
	}
}

func (me *impl) flushAndCloseRedirectionBatches() {
	if len(*me.currentRedirectionBatch) > 0 {
		me.redirectionBatches <- me.currentRedirectionBatch
		me.lastRedirectionFlush = time.Now()
	} else {
		me.recycleRedirectionBatch(me.currentRedirectionBatch)
	}
	close(me.redirectionBatches)
}

func (me *impl) redirectionRecordsBufferJob(ctx context.Context) error {
	ticker := time.NewTicker(redirectionFlushInterval)
	defer ticker.Stop()

	me.lastRedirectionFlush = time.Now()

	for {
		select {
		case <-ctx.Done():
			for record := range me.redirectionRecords {
				me.addToRedirectionBatch(record)
			}
			me.flushAndCloseRedirectionBatches()
			return nil
		case record, ok := <-me.redirectionRecords:
			if !ok {
				me.flushAndCloseRedirectionBatches()
				return nil
			}
			me.addToRedirectionBatch(record)
		case <-ticker.C:
			if len(*me.currentRedirectionBatch) > 0 && time.Since(me.lastRedirectionFlush) >= redirectionFlushInterval {
				me.redirectionBatches <- me.currentRedirectionBatch
				me.currentRedirectionBatch = me.redirectionPool.Get().(*[]redirectionRecord)
				me.lastRedirectionFlush = time.Now()
			}
		}
	}
}

func (me *impl) redirectionRecordsFlusherJob(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			for batch := range me.redirectionBatches {
				me.flushRedirectionBatch(*batch)
				me.recycleRedirectionBatch(batch)
			}
			return nil
		case batch, ok := <-me.redirectionBatches:
			if !ok {
				return nil
			}
			me.flushRedirectionBatch(*batch)
			me.recycleRedirectionBatch(batch)
		}
	}
}

func (me *impl) Record(ctx context.Context, urlId, userId uuid.UUID, ip, platform, os string) error {
	record := redirectionRecord{
		Id:        uuid.NewV7(),
		UrlId:     urlId,
		UserId:    userId,
		Ip:        ip,
		Platform:  platform,
		Os:        os,
		CreatedAt: time.Now(),
	}

	select {
	case me.redirectionRecords <- record:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("failed to buffer redirection: %w", ctx.Err())
	}
}

// Redirection is a recorded click on a short link.
type Redirection struct {
	Id        uuid.UUID
	UrlId     uuid.UUID
	UserId    uuid.UUID
	Ip        string
	Os        string
	Platform  string
	CreatedAt time.Time
}

// RedirectionsSort is a sort order for [Service.GetUserRedirections] and
// [Service.GetUrlRedirections].
type RedirectionsSort string

const (
	SortCreatedAtAscending  RedirectionsSort = "created_at.asc"
	SortCreatedAtDescending RedirectionsSort = "created_at.desc"
	SortOsAscending         RedirectionsSort = "os.asc"
	SortOsDescending        RedirectionsSort = "os.desc"
	SortPlatformAscending   RedirectionsSort = "platform.asc"
	SortPlatformDescending  RedirectionsSort = "platform.desc"
)

func (me RedirectionsSort) isValid() bool {
	switch me {
	case SortCreatedAtAscending, SortCreatedAtDescending,
		SortOsAscending, SortOsDescending,
		SortPlatformAscending, SortPlatformDescending:
		return true
	}
	return false
}

func toRedirection(r queries.RedirectionRedirection) Redirection {
	return Redirection{
		Id:        r.Id,
		UrlId:     r.UrlId,
		UserId:    r.UserId,
		Ip:        r.Ip,
		Os:        r.Os,
		Platform:  r.Platform,
		CreatedAt: r.CreatedAt,
	}
}

func (me *impl) GetUserRedirections(
	ctx context.Context,
	userId uuid.UUID,
	sort RedirectionsSort,
	lastCreatedAt time.Time, lastOs, lastPlatform string, lastRedirectionId uuid.UUID,
	limit int,
) ([]Redirection, error) {
	if !sort.isValid() {
		return nil, ErrInvalidSort
	}

	rows, err := me.queries.GetUserRedirections(ctx, queries.GetUserRedirectionsParams{
		UserId:            userId,
		Sort:              string(sort),
		LastCreatedAt:     utils.NullTime(lastCreatedAt),
		LastOs:            utils.NullString(lastOs),
		LastPlatform:      utils.NullString(lastPlatform),
		LastRedirectionId: utils.UuidPtr(lastRedirectionId),
		Limit:             int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user redirections: %w", err)
	}

	redirections := make([]Redirection, 0, len(rows))
	for _, r := range rows {
		redirections = append(redirections, toRedirection(r))
	}

	return redirections, nil
}

func (me *impl) GetRedirectionById(ctx context.Context, redirectionId, userId uuid.UUID) (*Redirection, error) {
	row, err := me.queries.GetRedirectionById(ctx, queries.GetRedirectionByIdParams{
		Id:     redirectionId,
		UserId: userId,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRedirectionNotFound
		}
		return nil, fmt.Errorf("failed to get redirection by id: %w", err)
	}

	return new(toRedirection(row)), nil
}

func (me *impl) GetUrlRedirections(
	ctx context.Context,
	urlId uuid.UUID,
	sort RedirectionsSort,
	lastCreatedAt time.Time, lastOs, lastPlatform string, lastRedirectionId uuid.UUID,
	limit int,
) ([]Redirection, error) {
	if !sort.isValid() {
		return nil, ErrInvalidSort
	}

	rows, err := me.queries.GetUrlRedirections(ctx, queries.GetUrlRedirectionsParams{
		UrlId:             urlId,
		Sort:              string(sort),
		LastCreatedAt:     utils.NullTime(lastCreatedAt),
		LastOs:            utils.NullString(lastOs),
		LastPlatform:      utils.NullString(lastPlatform),
		LastRedirectionId: utils.UuidPtr(lastRedirectionId),
		Limit:             int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get url redirections: %w", err)
	}

	redirections := make([]Redirection, 0, len(rows))
	for _, r := range rows {
		redirections = append(redirections, toRedirection(r))
	}

	return redirections, nil
}

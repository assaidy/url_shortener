package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"time"

	"github.com/assaidy/url_shortener/cache"
	"github.com/assaidy/url_shortener/config"
	"github.com/assaidy/url_shortener/db/postgres"
	"github.com/assaidy/url_shortener/repository/postgres"
	"github.com/assaidy/url_shortener/utils"
	"github.com/valkey-io/valkey-go"
)

var UrlServiceInstance = &UrlService{}

type UrlService struct {
	db      *sql.DB
	queries *postgres_repo.Queries
	cache   valkey.Client

	urlVisitChan       chan UrlVisit
	urlVisitWorkerDone chan struct{}
}

func (me *UrlService) Start() error {
	me.db = postgres_db.DB
	me.queries = postgres_repo.New(me.db)
	me.cache = cache.Valkey

	me.urlVisitChan = make(chan UrlVisit, 10_000)
	me.urlVisitWorkerDone = make(chan struct{}, 1)
	me.startUrlVisitWorker()

	return nil
}

func (me *UrlService) Stop() {
	close(me.urlVisitChan)
	<-me.urlVisitWorkerDone
}

type UrlVisit struct {
	ShorUrl   string
	VisitorIp string
	VisitedAt time.Time
}

func (me *UrlService) startUrlVisitWorker() {
	go func() {
		buffCap := 1000
		buffIndex := 0
		buff := make([]UrlVisit, buffCap)

		for visit := range me.urlVisitChan {
			buff[buffIndex] = visit
			buffIndex += 1

			if buffIndex == buffCap {
				me.flushUrlVisitBuffer(buff)
				buffIndex = 0
			}
		}
		me.flushUrlVisitBuffer(buff[0:buffIndex])

		me.urlVisitWorkerDone <- struct{}{}
	}()
}

func (me *UrlService) flushUrlVisitBuffer(buff []UrlVisit) {
	if len(buff) > 0 {
		for _, it := range buff {
			// TODO: this is terrible. use bulk insert
			if err := me.queries.InsertUrlVisits(context.Background(), postgres_repo.InsertUrlVisitsParams{
				ShortUrl:  it.ShorUrl,
				VisitorIp: it.VisitorIp,
				VisitedAt: it.VisitedAt,
			}); err != nil {
				slog.Error("error inserting url visits", "err", err)
			}
		}
		slog.Info("url visits stored successfully" , "PID", os.Getpid())
	}
}

type CreateShortUrlParams struct {
	Username string `validate:"required"`
	LongUrl  string `validate:"required,url"`
	ShortUrl string `validate:"customShortUrl"`
}

func (me *UrlService) CreateShortUrl(ctx context.Context, params CreateShortUrlParams) (string, error) {
	if err := utils.ValidateStruct(params); err != nil {
		return "", fmt.Errorf("%w: %s", ValidationErr, err.Error())
	}

	tx, err := me.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error beginning tx: %w", err)
	}
	defer tx.Rollback()
	qtx := me.queries.WithTx(tx)

	shortUrl := params.ShortUrl
	if shortUrl != "" {
		if ok, err := qtx.CheckShortUrl(ctx, shortUrl); err != nil {
			return "", fmt.Errorf("error checking short url: %w", err)
		} else if ok {
			return "", fmt.Errorf("%w: short url already exists", ConflictErr)
		}
	} else {
		shortUrlLength, err := qtx.GetShortUrlLength(ctx)
		if err != nil {
			return "", fmt.Errorf("error getting short url length: %w", err)
		}

		success := false
		for {
			for i := 0; i < config.RandomUrlCollisionRetries && !success; i++ {
				shortUrl = generateRandomShortUrl(int(shortUrlLength))

				if ok, err := qtx.CheckShortUrl(ctx, shortUrl); err != nil {
					return "", fmt.Errorf("error checking short url: %w", err)
				} else if !ok {
					success = true
				}
			}
			if success {
				break
			}
			newlength, err := qtx.IncrementShortUrlLength(ctx)
			if err != nil {
				return "", fmt.Errorf("error incrementing short url length: %w", err)
			}
			shortUrlLength += newlength
		}
	}

	if err := qtx.InsertShortUrl(ctx, postgres_repo.InsertShortUrlParams{
		Username: params.Username,
		LongUrl:  params.LongUrl,
		ShortUrl: shortUrl,
	}); err != nil {
		return "", fmt.Errorf("error inserting short url: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("error commiting tx: %w", err)
	}

	return shortUrl, nil
}

func generateRandomShortUrl(length int) string {
	charRange := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charRangeLength := len(charRange)
	buf := make([]byte, length)
	for i := range length {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(charRangeLength)))
		buf[i] = charRange[n.Int64()]
	}
	return string(buf)
}
func (me *UrlService) GetLongUrl(ctx context.Context, shortUrl string) (string, error) {
	val, err := me.cache.Do(ctx, me.cache.B().Get().Key(shortUrl).Build()).AsBytes()
	if err == nil && val != nil {
		return string(val), nil
	}

	slog.Warn("cache miss", "key", shortUrl)

	longUrl, err := me.queries.GetLongUrl(ctx, shortUrl)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: url not found", NotFoundErr)
		}
		return "", fmt.Errorf("error getting long url: %w", err)
	}

	if _, err := me.cache.Do(
		ctx,
		me.cache.B().
			Set().
			Key(shortUrl).
			Value(longUrl).
			Ex(config.CacheTTL).
			Build(),
	).AsBytes(); err != nil {
		slog.Error("error setting cache", "key", shortUrl, "value", longUrl, "err", err)
	}

	return longUrl, nil
}

func (me *UrlService) StoreUrlVisit(visit UrlVisit) {
	me.urlVisitChan <- visit
}

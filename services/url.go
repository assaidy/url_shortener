package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/assaidy/url_shortener/config"
	"github.com/assaidy/url_shortener/db"
	"github.com/assaidy/url_shortener/repository"
	"github.com/assaidy/url_shortener/utils"
)

var UrlServiceInstance = &UrlService{}

type UrlService struct {
	urlVisitChan       chan UrlVisit
	urlVisitWorkerDone chan struct{}
}

func (me *UrlService) Start() error {
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
				flushUrlVisitBuffer(buff)
				buffIndex = 0
			}
		}
		flushUrlVisitBuffer(buff[0:buffIndex])

		me.urlVisitWorkerDone <- struct{}{}
	}()
}

func flushUrlVisitBuffer(buff []UrlVisit) {
	if len(buff) > 0 {
		query := generateUrlVisitQuery(buff)
		if _, err := db.Connection.ExecContext(context.Background(), query); err != nil {
			slog.Error("error inserting url visits", "err", err)
		}
	}
}

func generateUrlVisitQuery(buff []UrlVisit) string {
	builder := make([]string, len(buff)+1)
	builder = append(builder, "insert into url_visits (short_url, visitor_ip, visited_at) values")
	for _, it := range buff {
		builder = append(builder, fmt.Sprintf("(%s, %s, %s)", it.ShorUrl, it.VisitorIp, it.VisitedAt))
	}
	builder = append(builder, ";")
	return strings.Join(builder, "\n")
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

	tx, err := db.Connection.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error beginning tx: %w", err)
	}
	defer tx.Rollback()
	qtx := queries.WithTx(tx)

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

	if err := qtx.InsertShortUrl(ctx, repository.InsertShortUrlParams{
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
	// TODO: lookup cache first
	longUrl, err := queries.GetLongUrl(ctx, shortUrl)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: url not found", NotFoundErr)
		}
		return "", fmt.Errorf("error getting long url: %w", err)
	}

	return longUrl, nil
}

func (me *UrlService) StoreUrlVisit(visit UrlVisit) {
	me.urlVisitChan <- visit
}

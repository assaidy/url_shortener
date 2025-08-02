package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/assaidy/url_shortener/config"
	"github.com/assaidy/url_shortener/db"
	"github.com/assaidy/url_shortener/repository"
	"github.com/assaidy/url_shortener/utils"
)

var UrlServiceInstance = &UrlService{}

type UrlService struct{}

func (me *UrlService) Start() error {
	slog.Info("url service started")
	return nil
}

func (me *UrlService) Stop() {
	slog.Info("url service stopped")
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

	// TODO: collect analytics

	return longUrl, nil
}

package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/assaidy/moon"
	"github.com/assaidy/moon/handlers/pagination"
	shortening_service "github.com/assaidy/url_shortener/src/services/shortening"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
)

func CreateUrl(ctx *moon.Context) error {
	var request struct {
		Url       string    `json:"url"`
		Title     string    `json:"title"`
		ExpiresAt time.Time `json:"expiresAt"`
	}
	if err := ctx.ReadJson(&request); err != nil {
		return ErrInvalidJson
	}

	request.Url = strings.TrimSpace(request.Url)
	request.Title = strings.TrimSpace(request.Title)

	if err := validation.ValidateStruct(&request,
		validation.Field(&request.Url, validation.Required, is.URL),
		validation.Field(&request.Title, validation.Length(1, 256)),
	); err != nil {
		return ErrInvalidPayload.WithDetails(err)
	}

	session := getCurrentSession(ctx)

	shorteningService := ctx.GetService[*shortening_service.Service]()
	url, err := shorteningService.CreateUrl(ctx, session.UserId, request.Title, request.Url, request.ExpiresAt)
	if err != nil {
		return err
	}

	return ctx.WriteJson(http.StatusCreated, urlObject(*url))
}

func urlObject(url shortening_service.Url) moon.Object {
	return moon.Object{
		"id":        url.Id,
		"userId":    url.UserId,
		"title":     url.Title,
		"url":       url.Url,
		"alias":     url.Alias,
		"createdAt": url.CreatedAt,
		"expiresAt": url.ExpiresAt,
		"isActive":  url.IsActive,
		"clicks":    url.Clicks,
	}
}

func GetOneUrl(ctx *moon.Context) error {
	urlId, err := uuid.Parse(ctx.GetParam("id"))
	if err != nil {
		return ErrUrlNotFound
	}

	session := getCurrentSession(ctx)

	shorteningService := ctx.GetService[*shortening_service.Service]()
	url, err := shorteningService.GetUserUrl(ctx, session.UserId, urlId)
	if err != nil {
		if errors.Is(err, shortening_service.ErrUrlNotFound) {
			return ErrUrlNotFound
		}
		return err
	}

	return ctx.WriteJson(http.StatusOK, urlObject(*url))
}

func EditUrl(ctx *moon.Context) error {
	urlId, err := uuid.Parse(ctx.GetParam("id"))
	if err != nil {
		return ErrUrlNotFound
	}

	var request struct {
		Title     string    `json:"title"`
		ExpiresAt time.Time `json:"expiresAt"`
		IsActive  bool      `json:"isActive"`
	}
	if err := ctx.ReadJson(&request); err != nil {
		return ErrInvalidJson
	}

	request.Title = strings.TrimSpace(request.Title)

	if err := validation.ValidateStruct(&request,
		validation.Field(&request.Title, validation.Length(1, 256)),
	); err != nil {
		return ErrInvalidPayload.WithDetails(err)
	}

	session := getCurrentSession(ctx)

	shorteningService := ctx.GetService[*shortening_service.Service]()
	url, err := shorteningService.UpdateUserUrl(ctx, session.UserId, urlId, request.Title, request.ExpiresAt, request.IsActive)
	if err != nil {
		if errors.Is(err, shortening_service.ErrUrlNotFound) {
			return ErrUrlNotFound
		}
		return err
	}

	return ctx.WriteJson(http.StatusOK, urlObject(*url))
}

func DeleteUrl(ctx *moon.Context) error {
	urlId, err := uuid.Parse(ctx.GetParam("id"))
	if err != nil {
		return ErrUrlNotFound
	}

	session := getCurrentSession(ctx)

	shorteningService := ctx.GetService[*shortening_service.Service]()
	if err := shorteningService.DeleteUserUrl(ctx, session.UserId, urlId); err != nil {
		if errors.Is(err, shortening_service.ErrUrlNotFound) {
			return ErrUrlNotFound
		}
		return err
	}

	return ctx.WriteStatus(http.StatusOK)
}

func GetUrls(ctx *moon.Context) error {
	type cursor struct {
		LastTitle     string    `json:"lastTitle"`
		LastUrl       string    `json:"lastUrl"`
		LastAlias     string    `json:"lastAlias"`
		LastCreatedAt time.Time `json:"lastCreatedAt"`
		LastId        uuid.UUID `json:"lastId"`
	}

	paginationInfo, ok := ctx.GetLocal[*pagination.Info](pagination.DefaultLocalKey)
	moon.Assert(ok)

	var requestCursor cursor
	if encodedCursor := paginationInfo.GetCursor(); encodedCursor != "" {
		if err := pagination.DecodeCursor([]byte(encodedCursor), &requestCursor); err != nil {
			return ErrInvalidCursor
		}
	}

	// Ensure the cursor is either empty (first request) or fully populated (app-encoded cursor).
	if requestCursor != (cursor{}) &&
		(requestCursor.LastTitle == "" ||
			requestCursor.LastUrl == "" ||
			requestCursor.LastAlias == "" ||
			requestCursor.LastCreatedAt.IsZero() ||
			requestCursor.LastId == uuid.Nil()) {
		return ErrInvalidCursor
	}

	sort := paginationInfo.GetSort()
	if len(sort) != 1 {
		return moon.ErrBadRequest.WithDetails("only one sort key is allowed")
	}

	shorteningService := ctx.GetService[*shortening_service.Service]()
	urls, err := shorteningService.GetUserUrls(
		ctx,
		getCurrentSession(ctx).UserId,
		ctx.GetQuery("search"),
		getUserUrlsSort(sort[0]), // len == 1 always.
		requestCursor.LastTitle,
		requestCursor.LastUrl,
		requestCursor.LastAlias,
		requestCursor.LastCreatedAt,
		requestCursor.LastId,
		paginationInfo.GetLimit(),
	)
	if err != nil {
		return err
	}

	items := make([]moon.Object, 0, len(urls))
	for _, u := range urls {
		items = append(items, urlObject(u))
	}

	var nextCursor string
	if len(items) == paginationInfo.GetLimit() {
		last := urls[len(urls)-1]
		raw, err := pagination.EncodeCursor(cursor{
			LastTitle:     last.Title,
			LastUrl:       last.Url,
			LastAlias:     last.Alias,
			LastCreatedAt: last.CreatedAt,
			LastId:        last.Id,
		})
		if err != nil {
			return err
		}
		nextCursor = string(raw)
	}

	return ctx.WriteJson(http.StatusOK, moon.Object{
		"items":      items,
		"nextCursor": nextCursor,
	})
}

func getUserUrlsSort(sort pagination.Sort) shortening_service.UserUrlsSort {
	switch sort.Key {
	case "title":
		if sort.Order == pagination.OrderAscending {
			return shortening_service.SortTitleAscending
		}
		return shortening_service.SortTitleDescending
	case "url":
		if sort.Order == pagination.OrderAscending {
			return shortening_service.SortUrlAscending
		}
		return shortening_service.SortUrlDescending
	case "alias":
		if sort.Order == pagination.OrderAscending {
			return shortening_service.SortAliasAscending
		}
		return shortening_service.SortAliasDescending
	case "created_at":
		if sort.Order == pagination.OrderAscending {
			return shortening_service.SortCreatedAtAscending
		}
		return shortening_service.SortCreatedAtDescending
	}
	return ""
}

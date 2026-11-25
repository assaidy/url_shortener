package handlers

import (
	"errors"
	"net"
	"net/http"
	"time"
	"uuid"

	"github.com/assaidy/moon"
	"github.com/assaidy/moon/handlers/pagination"
	redirection_service "github.com/assaidy/url_shortener/src/services/redirection"
	shortening_service "github.com/assaidy/url_shortener/src/services/shortening"
	"github.com/assaidy/url_shortener/src/utils"
)

func Redirect(ctx *moon.Context) error {
	alias := ctx.GetParam("alias")

	shorteningService := ctx.GetService[*shortening_service.Service]()
	url, err := shorteningService.GetUrlByAlias(ctx, alias)
	if err != nil {
		if errors.Is(err, shortening_service.ErrUrlNotFound) {
			return moon.ErrNotFound
		}
		return err
	}

	if !url.IsActive || (url.ExpiresAt != nil && !time.Now().Before(*url.ExpiresAt)) {
		return moon.ErrGone
	}

	// Record the click before redirecting. A failed write must not prevent the
	// user from reaching the destination, so the error is only logged.
	//
	// The URL is intentionally not locked and the redirection table has no
	// foreign key to shortening.urls: the owner may soft-delete the URL while
	// this click is in flight, and we still want the redirection recorded.
	// Redirections are analytics history that outlives the URLs they point to.
	userAgent := ctx.GetHeader("User-Agent")
	redirectionService := ctx.GetService[*redirection_service.Service]()
	if err := redirectionService.Record(
		ctx,
		url.Id,
		url.UserId,
		clientIp(ctx),
		utils.ExtractPlatformFromUserAgent(userAgent),
		utils.ExtractOsFromUserAgent(userAgent),
	); err != nil {
		ctx.GetAppLogger().Error("failed to record redirection", "alias", alias, "error", err)
	}

	return ctx.Redirect(http.StatusFound, url.Url)
}

// clientIp returns the IP address of the client without the port. When the
// request address is not in host:port form it is returned unchanged, and when
// it is empty the IP is reported as "unknown".
func clientIp(ctx *moon.Context) string {
	address := ctx.GetRemoteAddress()
	if address == "" {
		return "unknown"
	}
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return address
}

func redirectionObject(redirection redirection_service.Redirection) moon.Object {
	return moon.Object{
		"id":        redirection.Id,
		"urlId":     redirection.UrlId,
		"userId":    redirection.UserId,
		"ip":        redirection.Ip, // FIX: i shouldn't return this; instead, use it to return the country.
		"platform":  redirection.Platform,
		"os":        redirection.Os,
		"createdAt": redirection.CreatedAt,
	}
}

type redirectionCursor struct {
	LastCreatedAt     time.Time `json:"lastCreatedAt"`
	LastOs            string    `json:"lastOs"`
	LastPlatform      string    `json:"lastPlatform"`
	LastRedirectionId uuid.UUID `json:"lastRedirectionId"`
}

// readRedirectionCursor decodes the pagination cursor. A zero cursor means the
// first page; any other cursor must be fully populated, because only the app
// creates cursors.
func readRedirectionCursor(paginationInfo *pagination.Info) (redirectionCursor, error) {
	var cursor redirectionCursor
	if encodedCursor := paginationInfo.GetCursor(); encodedCursor != "" {
		if err := pagination.DecodeCursor([]byte(encodedCursor), &cursor); err != nil {
			return redirectionCursor{}, ErrInvalidCursor
		}
	}

	if cursor != (redirectionCursor{}) &&
		(cursor.LastCreatedAt.IsZero() ||
			cursor.LastOs == "" ||
			cursor.LastPlatform == "" ||
			cursor.LastRedirectionId == uuid.Nil()) {
		return redirectionCursor{}, ErrInvalidCursor
	}

	return cursor, nil
}

func getRedirectionsSort(sort pagination.Sort) redirection_service.RedirectionsSort {
	switch sort.Key {
	case "created_at":
		if sort.Order == pagination.OrderAscending {
			return redirection_service.SortCreatedAtAscending
		}
		return redirection_service.SortCreatedAtDescending
	case "os":
		if sort.Order == pagination.OrderAscending {
			return redirection_service.SortOsAscending
		}
		return redirection_service.SortOsDescending
	case "platform":
		if sort.Order == pagination.OrderAscending {
			return redirection_service.SortPlatformAscending
		}
		return redirection_service.SortPlatformDescending
	}
	return ""
}

func nextRedirectionCursor(redirections []redirection_service.Redirection) (string, error) {
	last := redirections[len(redirections)-1]
	raw, err := pagination.EncodeCursor(redirectionCursor{
		LastCreatedAt:     last.CreatedAt,
		LastOs:            last.Os,
		LastPlatform:      last.Platform,
		LastRedirectionId: last.Id,
	})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func GetRedirections(ctx *moon.Context) error {
	paginationInfo, ok := ctx.GetLocal[*pagination.Info](pagination.DefaultLocalKey)
	moon.Assert(ok)

	cursor, err := readRedirectionCursor(paginationInfo)
	if err != nil {
		return err
	}

	sort := paginationInfo.GetSort()
	if len(sort) != 1 {
		return moon.ErrBadRequest.WithDetails("only one sort key is allowed")
	}

	redirectionService := ctx.GetService[*redirection_service.Service]()
	redirections, err := redirectionService.GetUserRedirections(
		ctx,
		getCurrentSession(ctx).UserId,
		getRedirectionsSort(sort[0]), // len == 1 always.
		cursor.LastCreatedAt,
		cursor.LastOs,
		cursor.LastPlatform,
		cursor.LastRedirectionId,
		paginationInfo.GetLimit(),
	)
	if err != nil {
		return err
	}

	items := make([]moon.Object, 0, len(redirections))
	for _, r := range redirections {
		items = append(items, redirectionObject(r))
	}

	var nextCursor string
	if len(items) == paginationInfo.GetLimit() {
		nextCursor, err = nextRedirectionCursor(redirections)
		if err != nil {
			return err
		}
	}

	return ctx.WriteJson(http.StatusOK, moon.Object{
		"items":      items,
		"nextCursor": nextCursor,
	})
}

func GetOneRedirection(ctx *moon.Context) error {
	id, err := uuid.Parse(ctx.GetParam("id"))
	if err != nil {
		return ErrRedirectionNotFound
	}

	redirectionService := ctx.GetService[*redirection_service.Service]()
	redirection, err := redirectionService.GetRedirectionById(ctx, id, getCurrentSession(ctx).UserId)
	if err != nil {
		if errors.Is(err, redirection_service.ErrRedirectionNotFound) {
			return ErrRedirectionNotFound
		}
		return err
	}

	return ctx.WriteJson(http.StatusOK, redirectionObject(*redirection))
}

func GetUrlRedirections(ctx *moon.Context) error {
	urlId, err := uuid.Parse(ctx.GetParam("id"))
	if err != nil {
		return ErrUrlNotFound
	}

	paginationInfo, ok := ctx.GetLocal[*pagination.Info](pagination.DefaultLocalKey)
	moon.Assert(ok)

	cursor, err := readRedirectionCursor(paginationInfo)
	if err != nil {
		return err
	}

	sort := paginationInfo.GetSort()
	if len(sort) != 1 {
		return moon.ErrBadRequest.WithDetails("only one sort key is allowed")
	}

	// Make sure the url belongs to the current user before exposing its
	// redirections. The url may be deleted after this check; redirections are
	// analytics history and remain readable in that case.
	shorteningService := ctx.GetService[*shortening_service.Service]()
	if _, err := shorteningService.GetUserUrl(ctx, getCurrentSession(ctx).UserId, urlId); err != nil {
		if errors.Is(err, shortening_service.ErrUrlNotFound) {
			return ErrUrlNotFound
		}
		return err
	}

	redirectionService := ctx.GetService[*redirection_service.Service]()
	redirections, err := redirectionService.GetUrlRedirections(
		ctx,
		urlId,
		getRedirectionsSort(sort[0]), // len == 1 always.
		cursor.LastCreatedAt,
		cursor.LastOs,
		cursor.LastPlatform,
		cursor.LastRedirectionId,
		paginationInfo.GetLimit(),
	)
	if err != nil {
		return err
	}

	items := make([]moon.Object, 0, len(redirections))
	for _, r := range redirections {
		items = append(items, redirectionObject(r))
	}

	var nextCursor string
	if len(items) == paginationInfo.GetLimit() {
		nextCursor, err = nextRedirectionCursor(redirections)
		if err != nil {
			return err
		}
	}

	return ctx.WriteJson(http.StatusOK, moon.Object{
		"items":      items,
		"nextCursor": nextCursor,
	})
}

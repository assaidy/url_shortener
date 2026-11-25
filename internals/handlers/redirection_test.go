package handlers_test

import (
	"database/sql"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/assaidy/moon"
	"github.com/assaidy/moon/handlers/pagination"
	"github.com/stretchr/testify/require"
)

const redirectTestUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func doRedirect(t *testing.T, app *moon.App, alias string) *http.Response {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/r/"+alias, nil)
	req.Header.Set("User-Agent", redirectTestUserAgent)

	resp := app.Test(req)
	t.Cleanup(func() {
		_ = resp.Body.Close()
	})
	return resp
}

func countRedirections(db *sql.DB, urlID string) int {
	var count int
	if err := db.QueryRow(
		"select count(*) from redirection.redirections where url_id = $1",
		urlID,
	).Scan(&count); err != nil {
		return -1
	}
	return count
}

// requireRedirectionsFlushed waits for n redirections to be persisted for
// urlID. Redirections are written asynchronously by the buffered flusher.
func requireRedirectionsFlushed(t *testing.T, db *sql.DB, urlID string, n int) {
	t.Helper()

	require.Eventually(t, func() bool {
		return countRedirections(db, urlID) == n
	}, 5*time.Second, 50*time.Millisecond)
}

// latestRedirectionID returns the newest redirection recorded for urlID,
// waiting for the buffered flusher to persist it.
func latestRedirectionID(t *testing.T, db *sql.DB, urlID string) string {
	t.Helper()

	var id string
	require.Eventually(t, func() bool {
		err := db.QueryRow(
			`select id::text
			 from redirection.redirections
			 where url_id = $1
			 order by created_at desc
			 limit 1`,
			urlID,
		).Scan(&id)
		return err == nil && id != ""
	}, 5*time.Second, 50*time.Millisecond)
	return id
}

type redirectionDetail struct {
	ID        string    `json:"id"`
	URLID     string    `json:"urlId"`
	UserID    string    `json:"userId"`
	IP        string    `json:"ip"`
	Platform  string    `json:"platform"`
	Os        string    `json:"os"`
	CreatedAt time.Time `json:"createdAt"`
}

func TestRedirectSyncsClicks(t *testing.T) {
	t.Parallel()

	app, dbURL, _ := setupTestAppWithDB(t)
	cookies := registerAndLogin(t, app, "redirect_click_sync_user")

	created := createUrl(t, app, cookies, map[string]any{
		"url":   "https://example.com/click-sync",
		"title": "Click Sync",
	})
	require.Zero(t, created.Clicks)

	for range 2 {
		resp := doRedirect(t, app, created.Alias)
		require.Equal(t, http.StatusFound, resp.StatusCode)
	}

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.Eventually(t, func() bool {
		var clicks int64
		err := db.QueryRow(
			`select clicks from shortening.urls where id = $1`,
			created.ID,
		).Scan(&clicks)
		return err == nil && clicks == 2
	}, 5*time.Second, 50*time.Millisecond)

	require.Equal(t, int64(2), getUrl(t, app, cookies, created.ID).Clicks)
}

func TestRedirect(t *testing.T) {
	t.Parallel()

	app, dbURL, logs := setupTestAppWithDB(t)
	cookies := registerAndLogin(t, app, "redirect_user")

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	t.Run("returns not found for an unknown alias", func(t *testing.T) {
		resp := doRedirect(t, app, "doesnotexist")
		requireError(t, resp, http.StatusNotFound, "not_found")
	})

	t.Run("redirects to the destination", func(t *testing.T) {
		created := createUrl(t, app, cookies, map[string]any{
			"url":   "https://example.com/redirect-target",
			"title": "Redirect Target",
		})

		resp := doRedirect(t, app, created.Alias)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, created.URL, resp.Header.Get("Location"))

		requireRedirectionsFlushed(t, db, created.ID, 1)

		urlID, err := uuid.Parse(created.ID)
		require.NoError(t, err)

		var (
			storedURLID string
			ip          string
			platform    string
			os          string
		)
		require.NoError(t, db.QueryRow(
			`select url_id::text, ip, platform, os
			 from redirection.redirections
			 where url_id = $1
			 order by created_at desc
			 limit 1`,
			urlID.String(),
		).Scan(&storedURLID, &ip, &platform, &os))

		require.Equal(t, created.ID, storedURLID)
		require.Equal(t, "192.0.2.1", ip)
		require.Equal(t, "Chrome", platform)
		require.Equal(t, "Windows 10", os)
	})

	t.Run("returns gone for an expired url", func(t *testing.T) {
		created := createUrl(t, app, cookies, map[string]any{
			"url":       "https://example.com/expired",
			"title":     "Expired",
			"expiresAt": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano),
		})

		resp := doRedirect(t, app, created.Alias)
		requireError(t, resp, http.StatusGone, "gone")

		require.Zero(t, countRedirections(db, created.ID))
	})

	t.Run("returns gone for an inactive url", func(t *testing.T) {
		created := createUrl(t, app, cookies, map[string]any{
			"url":   "https://example.com/inactive",
			"title": "Inactive",
		})

		editUrl(t, app, cookies, created.ID, map[string]any{
			"title":     "Inactive",
			"expiresAt": nil,
			"isActive":  false,
		})

		resp := doRedirect(t, app, created.Alias)
		requireError(t, resp, http.StatusGone, "gone")

		require.Zero(t, countRedirections(db, created.ID))
	})

	t.Run("returns not found for a deleted url", func(t *testing.T) {
		created := createUrl(t, app, cookies, map[string]any{
			"url":   "https://example.com/deleted",
			"title": "Deleted",
		})

		resp := doJSON(t, app, http.MethodDelete, "/urls/"+created.ID, nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		resp = doRedirect(t, app, created.Alias)
		requireError(t, resp, http.StatusNotFound, "not_found")
	})

	t.Run("redirects and logs when recording fails", func(t *testing.T) {
		created := createUrl(t, app, cookies, map[string]any{
			"url":   "https://example.com/record-failure",
			"title": "Record Failure",
		})

		// Removing the table makes the buffered batch insert fail without
		// affecting URL resolution or the redirect response. The failure is
		// logged asynchronously by the flusher.
		_, err := db.Exec("drop table redirection.redirections")
		require.NoError(t, err)

		resp := doRedirect(t, app, created.Alias)
		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, created.URL, resp.Header.Get("Location"))

		var entry loggedEntry
		require.Eventually(t, func() bool {
			var ok bool
			entry, ok = logs.Find(slog.LevelError, "failed to insert redirection batch")
			return ok
		}, 5*time.Second, 50*time.Millisecond)
		require.Equal(t, "1", entry.Attrs["count"].String())
		require.Contains(t, entry.Attrs["error"].String(), "does not exist")
	})
}

type urlRedirectionsPage struct {
	Items      []redirectionDetail `json:"items"`
	NextCursor string              `json:"nextCursor"`
}

// listRedirections fetches a redirection listing page from path.
func listRedirections(t *testing.T, app *moon.App, path string, cookies []*http.Cookie, query url.Values) urlRedirectionsPage {
	t.Helper()

	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	resp := doJSON(t, app, http.MethodGet, path, nil, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return decodeJSON[urlRedirectionsPage](t, resp)
}

func listedRedirectionIDs(items []redirectionDetail) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

// redirectionIDsInDBOrder returns the IDs as PostgreSQL orders them with
// orderBy. It is the oracle for collation-sensitive sorts such as os.
func redirectionIDsInDBOrder(t *testing.T, db *sql.DB, filterColumn, filterValue, orderBy string) []string {
	t.Helper()

	rows, err := db.Query(
		"select id::text from redirection.redirections where "+filterColumn+" = $1 order by "+orderBy,
		filterValue,
	)
	require.NoError(t, err)
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	return ids
}

type redirectionSeed struct {
	ID        uuid.UUID
	URLID     string
	UserID    string
	IP        string
	OS        string
	Platform  string
	CreatedAt time.Time
}

// insertRedirections writes redirections directly, bypassing the asynchronous
// recorder so tests can control os, platform and created_at.
func insertRedirections(t *testing.T, db *sql.DB, seeds ...redirectionSeed) {
	t.Helper()

	for _, s := range seeds {
		_, err := db.Exec(
			`insert into redirection.redirections (id, url_id, user_id, ip, platform, os, created_at)
			 values ($1, $2, $3, $4, $5, $6, $7)`,
			s.ID, s.URLID, s.UserID, s.IP, s.Platform, s.OS, s.CreatedAt,
		)
		require.NoError(t, err)
	}
}

func TestGetUrlRedirections(t *testing.T) {
	t.Parallel()

	app, dbURL, _ := setupTestAppWithDB(t)
	cookies := registerAndLogin(t, app, "get_url_redirections_user")
	userID := currentAccountID(t, app, cookies)

	created := createUrl(t, app, cookies, map[string]any{
		"url":   "https://example.com/url-redirections",
		"title": "URL Redirections",
	})

	resp := doRedirect(t, app, created.Alias)
	require.Equal(t, http.StatusFound, resp.StatusCode)

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	requireRedirectionsFlushed(t, db, created.ID, 1)

	t.Run("rejects request without session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/urls/"+created.ID, nil)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("returns not found for an invalid id", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/urls/not-a-uuid", nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")
	})

	t.Run("returns not found for another user's url", func(t *testing.T) {
		otherCookies := registerAndLogin(t, app, "get_url_redirections_other_user")
		resp := doJSON(t, app, http.MethodGet, "/redirections/urls/"+created.ID, nil, otherCookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")
	})

	t.Run("returns the url redirections", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/urls/"+created.ID, nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		page := decodeJSON[urlRedirectionsPage](t, resp)
		require.Len(t, page.Items, 1)
		require.Equal(t, created.ID, page.Items[0].URLID)
		require.Empty(t, page.NextCursor)
	})

	t.Run("rejects multiple sort keys", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/urls/"+created.ID+"?sort=os,-platform", nil, cookies...)
		requireError(t, resp, http.StatusBadRequest, "bad_request")
	})

	t.Run("rejects unknown sort keys", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/urls/"+created.ID+"?sort=country", nil, cookies...)
		requireError(t, resp, http.StatusBadRequest, "bad_request")
	})

	t.Run("sorts by each allowed key", func(t *testing.T) {
		base := time.Now().UTC().Add(-time.Hour)
		insertRedirections(t, db,
			redirectionSeed{uuid.NewV7(), created.ID, userID, "192.0.2.21", "Linux", "Firefox", base.Add(time.Minute)},
			redirectionSeed{uuid.NewV7(), created.ID, userID, "192.0.2.22", "macOS", "Safari", base.Add(2 * time.Minute)},
			redirectionSeed{uuid.NewV7(), created.ID, userID, "192.0.2.23", "Linux", "Chrome", base.Add(3 * time.Minute)},
		)

		cases := []struct {
			name    string
			sort    string
			orderBy string
		}{
			{"created_at ascending", "created_at", "created_at asc, id desc"},
			{"created_at descending", "-created_at", "created_at desc, id desc"},
			{"os ascending", "os", "os asc, id desc"},
			{"os descending", "-os", "os desc, id desc"},
			{"platform ascending", "platform", "platform asc, id desc"},
			{"platform descending", "-platform", "platform desc, id desc"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				expected := redirectionIDsInDBOrder(t, db, "url_id", created.ID, tc.orderBy)
				page := listRedirections(t, app, "/redirections/urls/"+created.ID, cookies, url.Values{
					"limit": {"100"},
					"sort":  {tc.sort},
				})
				require.Len(t, page.Items, len(expected))
				require.Equal(t, expected, listedRedirectionIDs(page.Items))
			})
		}
	})

	t.Run("paginates sorted results with cursors", func(t *testing.T) {
		full := listRedirections(t, app, "/redirections/urls/"+created.ID, cookies, url.Values{
			"limit": {"100"},
			"sort":  {"os"},
		})

		var walked []redirectionDetail
		query := url.Values{"limit": {"2"}, "sort": {"os"}}
		pages := 0
		for {
			page := listRedirections(t, app, "/redirections/urls/"+created.ID, cookies, query)
			pages++
			require.Less(t, pages, 10, "pagination did not terminate")

			walked = append(walked, page.Items...)
			if page.NextCursor == "" {
				break
			}
			query = url.Values{"limit": {"2"}, "sort": {"os"}, "cursor": {page.NextCursor}}
		}

		require.Equal(t, 3, pages)
		require.Equal(t, listedRedirectionIDs(full.Items), listedRedirectionIDs(walked))
	})
}

func TestGetRedirections(t *testing.T) {
	t.Parallel()

	app, dbURL, _ := setupTestAppWithDB(t)
	cookies := registerAndLogin(t, app, "get_redirections_user")
	userID := currentAccountID(t, app, cookies)

	first := createUrl(t, app, cookies, map[string]any{
		"url":   "https://example.com/get-redirections-1",
		"title": "Get Redirections 1",
	})
	second := createUrl(t, app, cookies, map[string]any{
		"url":   "https://example.com/get-redirections-2",
		"title": "Get Redirections 2",
	})

	// Record three clicks: the first link twice, then the second link once.
	for _, created := range []createdUrl{first, first, second} {
		resp := doRedirect(t, app, created.Alias)
		require.Equal(t, http.StatusFound, resp.StatusCode)
	}

	otherCookies := registerAndLogin(t, app, "get_redirections_other_user")
	otherUrl := createUrl(t, app, otherCookies, map[string]any{
		"url":   "https://example.com/get-redirections-other",
		"title": "Other User's Link",
	})
	resp := doRedirect(t, app, otherUrl.Alias)
	require.Equal(t, http.StatusFound, resp.StatusCode)

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	requireRedirectionsFlushed(t, db, first.ID, 2)
	requireRedirectionsFlushed(t, db, second.ID, 1)
	requireRedirectionsFlushed(t, db, otherUrl.ID, 1)

	t.Run("rejects request without session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections", nil)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects an invalid cursor", func(t *testing.T) {
		cursor, err := pagination.EncodeCursor("not-a-cursor-object")
		require.NoError(t, err)

		query := url.Values{}
		query.Set("cursor", string(cursor))
		resp := doJSON(t, app, http.MethodGet, "/redirections?"+query.Encode(), nil, cookies...)
		requireError(t, resp, http.StatusBadRequest, "invalid_cursor")
	})

	t.Run("returns only the current user's redirections newest first", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections", nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		page := decodeJSON[urlRedirectionsPage](t, resp)
		require.Len(t, page.Items, 3)
		require.Empty(t, page.NextCursor)

		for _, item := range page.Items {
			require.Equal(t, userID, item.UserID)
			require.NotEqual(t, otherUrl.ID, item.URLID)
		}

		// The most recent click was on the second link.
		require.Equal(t, second.ID, page.Items[0].URLID)

		counts := make(map[string]int, 2)
		for _, item := range page.Items {
			counts[item.URLID]++
		}
		require.Equal(t, map[string]int{first.ID: 2, second.ID: 1}, counts)
	})

	t.Run("paginates with a cursor", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections?limit=2", nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		firstPage := decodeJSON[urlRedirectionsPage](t, resp)
		require.Len(t, firstPage.Items, 2)
		require.NotEmpty(t, firstPage.NextCursor)

		query := url.Values{}
		query.Set("limit", "2")
		query.Set("cursor", firstPage.NextCursor)
		resp = doJSON(t, app, http.MethodGet, "/redirections?"+query.Encode(), nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		secondPage := decodeJSON[urlRedirectionsPage](t, resp)
		require.Len(t, secondPage.Items, 1)
		require.Empty(t, secondPage.NextCursor)

		firstPageIDs := make([]string, 0, len(firstPage.Items))
		for _, item := range firstPage.Items {
			firstPageIDs = append(firstPageIDs, item.ID)
		}
		require.NotContains(t, firstPageIDs, secondPage.Items[0].ID)
	})

	t.Run("rejects multiple sort keys", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections?sort=os,-platform", nil, cookies...)
		requireError(t, resp, http.StatusBadRequest, "bad_request")
	})

	t.Run("rejects unknown sort keys", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections?sort=country", nil, cookies...)
		requireError(t, resp, http.StatusBadRequest, "bad_request")
	})

	t.Run("sorts by each allowed key", func(t *testing.T) {
		base := time.Now().UTC().Add(-time.Hour)
		insertRedirections(t, db,
			redirectionSeed{uuid.NewV7(), first.ID, userID, "192.0.2.31", "Linux", "Firefox", base.Add(time.Minute)},
			redirectionSeed{uuid.NewV7(), first.ID, userID, "192.0.2.32", "macOS", "Safari", base.Add(2 * time.Minute)},
			redirectionSeed{uuid.NewV7(), first.ID, userID, "192.0.2.33", "Linux", "Chrome", base.Add(3 * time.Minute)},
		)

		cases := []struct {
			name    string
			sort    string
			orderBy string
		}{
			{"created_at ascending", "created_at", "created_at asc, id desc"},
			{"created_at descending", "-created_at", "created_at desc, id desc"},
			{"os ascending", "os", "os asc, id desc"},
			{"os descending", "-os", "os desc, id desc"},
			{"platform ascending", "platform", "platform asc, id desc"},
			{"platform descending", "-platform", "platform desc, id desc"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				expected := redirectionIDsInDBOrder(t, db, "user_id", userID, tc.orderBy)
				page := listRedirections(t, app, "/redirections", cookies, url.Values{
					"limit": {"100"},
					"sort":  {tc.sort},
				})
				require.Len(t, page.Items, len(expected))
				require.Equal(t, expected, listedRedirectionIDs(page.Items))
			})
		}
	})

	t.Run("paginates sorted results with cursors", func(t *testing.T) {
		full := listRedirections(t, app, "/redirections", cookies, url.Values{
			"limit": {"100"},
			"sort":  {"platform"},
		})

		var walked []redirectionDetail
		query := url.Values{"limit": {"2"}, "sort": {"platform"}}
		pages := 0
		for {
			page := listRedirections(t, app, "/redirections", cookies, query)
			pages++
			require.Less(t, pages, 10, "pagination did not terminate")

			walked = append(walked, page.Items...)
			if page.NextCursor == "" {
				break
			}
			query = url.Values{"limit": {"2"}, "sort": {"platform"}, "cursor": {page.NextCursor}}
		}

		require.Equal(t, listedRedirectionIDs(full.Items), listedRedirectionIDs(walked))
	})
}

func TestGetRedirection(t *testing.T) {
	t.Parallel()

	app, dbURL, _ := setupTestAppWithDB(t)
	cookies := registerAndLogin(t, app, "get_redirection_user")
	userID := currentAccountID(t, app, cookies)

	created := createUrl(t, app, cookies, map[string]any{
		"url":   "https://example.com/get-redirection",
		"title": "Get Redirection",
	})

	resp := doRedirect(t, app, created.Alias)
	require.Equal(t, http.StatusFound, resp.StatusCode)

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	redirectionID := latestRedirectionID(t, db, created.ID)

	t.Run("rejects request without session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/"+redirectionID, nil)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("returns not found for an invalid id", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/not-a-uuid", nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "redirection_not_found")
	})

	t.Run("returns not found for an unknown redirection", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/"+uuid.NewV7().String(), nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "redirection_not_found")
	})

	t.Run("returns not found for another user's redirection", func(t *testing.T) {
		otherCookies := registerAndLogin(t, app, "get_redirection_other_user")
		resp := doJSON(t, app, http.MethodGet, "/redirections/"+redirectionID, nil, otherCookies...)
		requireError(t, resp, http.StatusNotFound, "redirection_not_found")

		resp = doJSON(t, app, http.MethodGet, "/redirections/"+redirectionID, nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns the redirection metadata", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/"+redirectionID, nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		detail := decodeJSON[redirectionDetail](t, resp)
		require.Equal(t, redirectionID, detail.ID)
		require.Equal(t, created.ID, detail.URLID)
		require.Equal(t, userID, detail.UserID)
		require.Equal(t, "192.0.2.1", detail.IP)
		require.Equal(t, "Chrome", detail.Platform)
		require.Equal(t, "Windows 10", detail.Os)
		require.False(t, detail.CreatedAt.IsZero())
		require.WithinDuration(t, time.Now(), detail.CreatedAt, time.Minute)
	})

	t.Run("returns the redirection object shape", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/redirections/"+redirectionID, nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		body := decodeJSON[map[string]any](t, resp)
		require.Equal(t,
			[]string{"createdAt", "id", "ip", "os", "platform", "urlId", "userId"},
			slices.Sorted(maps.Keys(body)),
		)
	})
}

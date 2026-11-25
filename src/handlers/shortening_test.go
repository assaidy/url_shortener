package handlers_test

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/assaidy/moon"
	"github.com/assaidy/moon/handlers/pagination"
	"github.com/stretchr/testify/require"
)

type createdUrl struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	Alias     string     `json:"alias"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
	IsActive  bool       `json:"isActive"`
	Clicks    int64      `json:"clicks"`
}

type listedUrl struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	Alias     string     `json:"alias"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
	IsActive  bool       `json:"isActive"`
	Clicks    int64      `json:"clicks"`
}

type urlsPage struct {
	Items      []listedUrl `json:"items"`
	NextCursor string      `json:"nextCursor"`
}

type urlDetail struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	Alias     string     `json:"alias"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
	IsActive  bool       `json:"isActive"`
	Clicks    int64      `json:"clicks"`
}

func currentAccountID(t *testing.T, app *moon.App, cookies []*http.Cookie) string {
	t.Helper()

	resp := doJSON(t, app, http.MethodGet, "/auth/me", nil, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	account := decodeJSON[struct {
		ID string `json:"id"`
	}](t, resp)
	require.NotEmpty(t, account.ID)
	return account.ID
}

func createUrl(t *testing.T, app *moon.App, cookies []*http.Cookie, body map[string]any) createdUrl {
	t.Helper()

	resp := doJSON(t, app, http.MethodPost, "/urls", body, cookies...)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	return decodeJSON[createdUrl](t, resp)
}

func listUrls(t *testing.T, app *moon.App, cookies []*http.Cookie, query url.Values) urlsPage {
	t.Helper()

	path := "/urls"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	resp := doJSON(t, app, http.MethodGet, path, nil, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return decodeJSON[urlsPage](t, resp)
}

func getUrl(t *testing.T, app *moon.App, cookies []*http.Cookie, id string) urlDetail {
	t.Helper()

	resp := doJSON(t, app, http.MethodGet, "/urls/"+id, nil, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return decodeJSON[urlDetail](t, resp)
}

func editUrl(t *testing.T, app *moon.App, cookies []*http.Cookie, id string, body map[string]any) urlDetail {
	t.Helper()

	resp := doJSON(t, app, http.MethodPut, "/urls/"+id, body, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return decodeJSON[urlDetail](t, resp)
}

// requireUrlShape asserts value uses the shared url object fields returned by
// every url endpoint.
func requireUrlShape(t *testing.T, value map[string]any) {
	t.Helper()

	require.Equal(t,
		[]string{"alias", "clicks", "createdAt", "expiresAt", "id", "isActive", "title", "url", "userId"},
		slices.Sorted(maps.Keys(value)),
	)
}

func listedUrlIDs(items []listedUrl) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func listedUrlTitles(items []listedUrl) []string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	return titles
}

// urlIDsInDBOrder returns the IDs of the user's urls as PostgreSQL orders them
// with orderBy. It is the oracle for collation-sensitive sorts such as alias.
func urlIDsInDBOrder(t *testing.T, dbURL, userID, orderBy string) []string {
	t.Helper()

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	rows, err := db.Query(
		"select id::text from shortening.urls where user_id = $1::uuid and deleted_at is null order by "+orderBy,
		userID,
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

func rawCursor(t *testing.T, raw string) string {
	t.Helper()

	return base64.URLEncoding.EncodeToString([]byte(raw))
}

func TestCreateUrl(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "create_url_user")
	userID := currentAccountID(t, app, cookies)

	validBody := func() map[string]any {
		return map[string]any{
			"url":   "https://example.com/page",
			"title": "Example Page",
		}
	}

	t.Run("rejects request without session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPost, "/urls", validBody())
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects request without csrf token", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPost, "/urls", validBody(), filterCookie(cookies, "csrf_token")...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects malformed json", func(t *testing.T) {
		resp := doRequest(t, app, http.MethodPost, "/urls", []byte("{"), cookies...)
		requireError(t, resp, http.StatusBadRequest, "invalid_json")
	})

	t.Run("rejects malformed expiresAt", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPost, "/urls", map[string]any{
			"url":       "https://example.com/page",
			"expiresAt": "not-a-timestamp",
		}, cookies...)
		requireError(t, resp, http.StatusBadRequest, "invalid_json")
	})

	t.Run("validates the payload", func(t *testing.T) {
		t.Run("missing url", func(t *testing.T) {
			resp := doJSON(t, app, http.MethodPost, "/urls", map[string]any{
				"title": "Missing URL",
			}, cookies...)
			requireError(t, resp, http.StatusUnprocessableEntity, "invalid_payload")
		})

		t.Run("blank url", func(t *testing.T) {
			resp := doJSON(t, app, http.MethodPost, "/urls", map[string]any{
				"url":   "   ",
				"title": "Blank URL",
			}, cookies...)
			requireError(t, resp, http.StatusUnprocessableEntity, "invalid_payload")
		})

		t.Run("invalid url", func(t *testing.T) {
			resp := doJSON(t, app, http.MethodPost, "/urls", map[string]any{
				"url":   "not-a-url",
				"title": "Invalid URL",
			}, cookies...)
			requireError(t, resp, http.StatusUnprocessableEntity, "invalid_payload")
		})

		t.Run("title too long", func(t *testing.T) {
			resp := doJSON(t, app, http.MethodPost, "/urls", map[string]any{
				"url":   "https://example.com/page",
				"title": strings.Repeat("a", 257),
			}, cookies...)
			requireError(t, resp, http.StatusUnprocessableEntity, "invalid_payload")
		})
	})

	t.Run("creates a url and trims its fields", func(t *testing.T) {
		before := time.Now().Add(-time.Second)

		created := createUrl(t, app, cookies, map[string]any{
			"url":   "  https://example.com/page  ",
			"title": "  Example Page  ",
		})

		require.NotEmpty(t, created.ID)
		require.Equal(t, userID, created.UserID)
		require.Equal(t, "https://example.com/page", created.URL)
		require.Equal(t, "Example Page", created.Title)
		require.NotEmpty(t, created.Alias)
		require.True(t, created.IsActive)
		require.Nil(t, created.ExpiresAt)
		require.WithinRange(t, created.CreatedAt, before, time.Now().Add(time.Second))
	})

	t.Run("allows an omitted title", func(t *testing.T) {
		created := createUrl(t, app, cookies, map[string]any{
			"url": "https://example.com/no-title",
		})
		require.Empty(t, created.Title)
	})

	t.Run("accepts a title at the maximum length", func(t *testing.T) {
		title := strings.Repeat("a", 256)
		created := createUrl(t, app, cookies, map[string]any{
			"url":   "https://example.com/max-title",
			"title": title,
		})
		require.Equal(t, title, created.Title)
	})

	t.Run("stores the expiration time", func(t *testing.T) {
		expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)

		created := createUrl(t, app, cookies, map[string]any{
			"url":       "https://example.com/expiring",
			"title":     "Expiring",
			"expiresAt": expiresAt.Format(time.RFC3339Nano),
		})

		require.NotNil(t, created.ExpiresAt)
		require.True(t, expiresAt.Equal(*created.ExpiresAt))
	})

	t.Run("generates a unique alias per url", func(t *testing.T) {
		aliases := make(map[string]struct{})
		for i := range 5 {
			created := createUrl(t, app, cookies, map[string]any{
				"url":   fmt.Sprintf("https://example.com/unique/%d", i),
				"title": fmt.Sprintf("Unique %d", i),
			})
			require.NotEmpty(t, created.Alias)
			require.NotContains(t, aliases, created.Alias)
			aliases[created.Alias] = struct{}{}
		}
	})
}

func TestGetUrls(t *testing.T) {
	t.Parallel()
	app, dbURL, _ := setupTestAppWithDB(t)
	cookies := registerAndLogin(t, app, "get_urls_user")
	userID := currentAccountID(t, app, cookies)

	seed := func(title, rawUrl string) createdUrl {
		return createUrl(t, app, cookies, map[string]any{
			"url":   rawUrl,
			"title": title,
		})
	}

	t.Run("rejects request without session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/urls", nil)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("returns an empty list when the user has no urls", func(t *testing.T) {
		page := listUrls(t, app, cookies, nil)
		require.Empty(t, page.Items)
		require.Empty(t, page.NextCursor)
	})

	first := seed("First", "https://example.com/first")
	second := seed("Second", "https://example.com/second")
	third := seed("Third", "https://example.com/third")

	t.Run("lists urls newest first", func(t *testing.T) {
		page := listUrls(t, app, cookies, nil)

		require.Len(t, page.Items, 3)
		require.Empty(t, page.NextCursor)
		require.Equal(t, []string{third.ID, second.ID, first.ID}, listedUrlIDs(page.Items))
	})

	t.Run("exposes the stored fields", func(t *testing.T) {
		page := listUrls(t, app, cookies, nil)
		require.Len(t, page.Items, 3)

		item := page.Items[0]
		require.Equal(t, third.ID, item.ID)
		require.Equal(t, userID, item.UserID)
		require.Equal(t, third.Title, item.Title)
		require.Equal(t, third.URL, item.URL)
		require.Equal(t, third.Alias, item.Alias)
		require.True(t, item.IsActive)
		require.Nil(t, item.ExpiresAt)
		require.WithinDuration(t, third.CreatedAt, item.CreatedAt, time.Millisecond)
	})

	t.Run("returns items with the shared url object shape", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/urls", nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		body := decodeJSON[struct {
			Items []map[string]any `json:"items"`
		}](t, resp)
		require.NotEmpty(t, body.Items)
		requireUrlShape(t, body.Items[0])
	})

	t.Run("scopes the list to the authenticated user", func(t *testing.T) {
		otherCookies := registerAndLogin(t, app, "get_urls_other_user")
		otherUrl := createUrl(t, app, otherCookies, map[string]any{
			"url":   "https://example.com/other",
			"title": "Other",
		})
		require.NotEqual(t, userID, otherUrl.UserID)

		page := listUrls(t, app, cookies, nil)
		require.Len(t, page.Items, 3)
		for _, item := range page.Items {
			require.Equal(t, userID, item.UserID)
		}

		otherPage := listUrls(t, app, otherCookies, nil)
		require.Len(t, otherPage.Items, 1)
		require.Equal(t, otherUrl.ID, otherPage.Items[0].ID)
	})

	// Seed enough urls to exercise multi-page pagination.
	for i := range 9 {
		seed(fmt.Sprintf("Bulk %d", i), fmt.Sprintf("https://example.com/bulk/%d", i))
	}

	t.Run("paginates with cursors", func(t *testing.T) {
		all := listUrls(t, app, cookies, url.Values{"limit": {"100"}})
		require.Len(t, all.Items, 12)
		require.Empty(t, all.NextCursor)

		var walked []listedUrl
		query := url.Values{"limit": {"5"}}
		pages := 0
		for {
			page := listUrls(t, app, cookies, query)
			pages++
			require.Less(t, pages, 10, "pagination did not terminate")

			walked = append(walked, page.Items...)
			if page.NextCursor == "" {
				break
			}
			query = url.Values{"limit": {"5"}, "cursor": {page.NextCursor}}
		}

		require.Equal(t, 3, pages)
		require.Len(t, walked, 12)
		require.Equal(t, listedUrlIDs(all.Items), listedUrlIDs(walked))
	})

	t.Run("uses the default limit", func(t *testing.T) {
		page := listUrls(t, app, cookies, nil)
		require.Len(t, page.Items, 10)
		require.NotEmpty(t, page.NextCursor)
	})

	t.Run("returns an empty page when the cursor is exhausted", func(t *testing.T) {
		full := listUrls(t, app, cookies, url.Values{"limit": {"12"}})
		require.Len(t, full.Items, 12)
		require.NotEmpty(t, full.NextCursor)

		page := listUrls(t, app, cookies, url.Values{
			"limit":  {"12"},
			"cursor": {full.NextCursor},
		})
		require.Empty(t, page.Items)
		require.Empty(t, page.NextCursor)
	})

	t.Run("treats the nil uuid cursor as the beginning", func(t *testing.T) {
		all := listUrls(t, app, cookies, url.Values{"limit": {"100"}})

		raw, err := pagination.EncodeCursor(struct {
			LastId uuid.UUID `json:"lastId"`
		}{LastId: uuid.Nil()})
		require.NoError(t, err)

		page := listUrls(t, app, cookies, url.Values{"cursor": {string(raw)}})
		require.Len(t, page.Items, 10)
		require.Equal(t, listedUrlIDs(all.Items[:10]), listedUrlIDs(page.Items))
	})

	t.Run("ignores page and offset query keys", func(t *testing.T) {
		all := listUrls(t, app, cookies, url.Values{"limit": {"100"}})

		page := listUrls(t, app, cookies, url.Values{
			"limit":  {"100"},
			"page":   {"99"},
			"offset": {"5"},
		})
		require.Equal(t, listedUrlIDs(all.Items), listedUrlIDs(page.Items))
	})

	t.Run("rejects invalid limits", func(t *testing.T) {
		cases := []struct {
			name  string
			limit string
		}{
			{"zero", "0"},
			{"negative", "-1"},
			{"above maximum", "101"},
			{"not a number", "abc"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				resp := doJSON(t, app, http.MethodGet, "/urls?limit="+tc.limit, nil, cookies...)
				requireError(t, resp, http.StatusBadRequest, "bad_request")
			})
		}
	})

	t.Run("rejects invalid cursors", func(t *testing.T) {
		t.Run("not base64", func(t *testing.T) {
			resp := doJSON(t, app, http.MethodGet, "/urls?cursor=!!!not-base64!!!", nil, cookies...)
			requireError(t, resp, http.StatusBadRequest, "bad_request")
		})

		t.Run("base64 encoded non-json", func(t *testing.T) {
			path := "/urls?" + url.Values{"cursor": {rawCursor(t, "not-json")}}.Encode()
			resp := doJSON(t, app, http.MethodGet, path, nil, cookies...)
			requireError(t, resp, http.StatusBadRequest, "bad_request")
		})

		t.Run("json with the wrong shape", func(t *testing.T) {
			cases := []struct {
				name string
				raw  string
			}{
				{"scalar", `"not-a-cursor"`},
				{"number", "123"},
				{"non-uuid lastId", `{"lastId":"not-a-uuid"}`},
				{"numeric lastId", `{"lastId":123}`},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					path := "/urls?" + url.Values{"cursor": {rawCursor(t, tc.raw)}}.Encode()
					resp := doJSON(t, app, http.MethodGet, path, nil, cookies...)
					requireError(t, resp, http.StatusBadRequest, "invalid_cursor")
				})
			}
		})
	})

	t.Run("sorts by each allowed key", func(t *testing.T) {
		cases := []struct {
			name    string
			sort    string
			orderBy string
		}{
			{"title ascending", "title", "title asc, id desc"},
			{"title descending", "-title", "title desc, id desc"},
			{"url ascending", "url", "url asc, id desc"},
			{"url descending", "-url", "url desc, id desc"},
			{"alias ascending", "alias", "alias asc, id desc"},
			{"alias descending", "-alias", "alias desc, id desc"},
			{"created_at ascending", "created_at", "created_at asc, id desc"},
			{"created_at descending", "-created_at", "created_at desc, id desc"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				expected := urlIDsInDBOrder(t, dbURL, userID, tc.orderBy)
				page := listUrls(t, app, cookies, url.Values{
					"limit": {"100"},
					"sort":  {tc.sort},
				})
				require.Len(t, page.Items, len(expected))
				require.Equal(t, expected, listedUrlIDs(page.Items))
			})
		}
	})

	t.Run("paginates sorted results with cursors", func(t *testing.T) {
		full := listUrls(t, app, cookies, url.Values{"limit": {"100"}, "sort": {"title"}})

		var walked []listedUrl
		query := url.Values{"limit": {"5"}, "sort": {"title"}}
		pages := 0
		for {
			page := listUrls(t, app, cookies, query)
			pages++
			require.Less(t, pages, 10, "pagination did not terminate")

			walked = append(walked, page.Items...)
			if page.NextCursor == "" {
				break
			}
			query = url.Values{"limit": {"5"}, "sort": {"title"}, "cursor": {page.NextCursor}}
		}

		require.Equal(t, 3, pages)
		require.Equal(t, listedUrlIDs(full.Items), listedUrlIDs(walked))
	})

	t.Run("searches titles", func(t *testing.T) {
		page := listUrls(t, app, cookies, url.Values{"limit": {"100"}, "search": {"Bulk"}})
		require.Len(t, page.Items, 9)
		for _, item := range page.Items {
			require.Contains(t, item.Title, "Bulk")
		}
	})

	t.Run("search is case insensitive", func(t *testing.T) {
		page := listUrls(t, app, cookies, url.Values{"limit": {"100"}, "search": {"bUlK"}})
		require.Len(t, page.Items, 9)
	})

	t.Run("search returns no matches", func(t *testing.T) {
		page := listUrls(t, app, cookies, url.Values{"search": {"does-not-exist"}})
		require.Empty(t, page.Items)
		require.Empty(t, page.NextCursor)
	})

	t.Run("search combines with sorting", func(t *testing.T) {
		page := listUrls(t, app, cookies, url.Values{
			"limit":  {"100"},
			"search": {"Bulk"},
			"sort":   {"-title"},
		})
		require.Equal(t, []string{
			"Bulk 8", "Bulk 7", "Bulk 6", "Bulk 5", "Bulk 4",
			"Bulk 3", "Bulk 2", "Bulk 1", "Bulk 0",
		}, listedUrlTitles(page.Items))
	})

	t.Run("search paginates with cursors", func(t *testing.T) {
		full := listUrls(t, app, cookies, url.Values{"limit": {"100"}, "search": {"Bulk"}})

		var walked []listedUrl
		query := url.Values{"limit": {"2"}, "search": {"Bulk"}}
		pages := 0
		for {
			page := listUrls(t, app, cookies, query)
			pages++
			require.Less(t, pages, 10, "pagination did not terminate")

			walked = append(walked, page.Items...)
			if page.NextCursor == "" {
				break
			}
			query = url.Values{"limit": {"2"}, "search": {"Bulk"}, "cursor": {page.NextCursor}}
		}

		require.Equal(t, 5, pages)
		require.Equal(t, listedUrlIDs(full.Items), listedUrlIDs(walked))
	})

	t.Run("rejects multiple sort keys", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/urls?sort=title,-url", nil, cookies...)
		requireError(t, resp, http.StatusBadRequest, "bad_request")
	})

	t.Run("rejects unknown sort keys", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/urls?sort=clicks", nil, cookies...)
		requireError(t, resp, http.StatusBadRequest, "bad_request")
	})
}

func TestGetUrl(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "get_url_user")
	userID := currentAccountID(t, app, cookies)

	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)
	created := createUrl(t, app, cookies, map[string]any{
		"url":       "https://example.com/detail",
		"title":     "Detail",
		"expiresAt": expiresAt.Format(time.RFC3339Nano),
	})

	t.Run("rejects request without session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/urls/"+created.ID, nil)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("returns not found for an invalid url id", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/urls/not-a-uuid", nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")
	})

	t.Run("returns not found for an unknown url", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/urls/"+uuid.NewV7().String(), nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")
	})

	t.Run("returns not found for another user's url", func(t *testing.T) {
		otherCookies := registerAndLogin(t, app, "get_url_other_user")
		otherUrl := createUrl(t, app, otherCookies, map[string]any{
			"url":   "https://example.com/other-detail",
			"title": "Other Detail",
		})

		resp := doJSON(t, app, http.MethodGet, "/urls/"+otherUrl.ID, nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")

		resp = doJSON(t, app, http.MethodGet, "/urls/"+otherUrl.ID, nil, otherCookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("returns the url metadata", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/urls/"+created.ID, nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		detail := decodeJSON[urlDetail](t, resp)
		require.Equal(t, created.ID, detail.ID)
		require.Equal(t, userID, detail.UserID)
		require.Equal(t, created.Title, detail.Title)
		require.Equal(t, created.URL, detail.URL)
		require.Equal(t, created.Alias, detail.Alias)
		require.True(t, detail.IsActive)
		require.NotNil(t, detail.ExpiresAt)
		require.True(t, expiresAt.Equal(*detail.ExpiresAt))
		require.WithinDuration(t, created.CreatedAt, detail.CreatedAt, time.Millisecond)
	})
}

func TestEditUrl(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "edit_url_user")
	userID := currentAccountID(t, app, cookies)

	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)
	created := createUrl(t, app, cookies, map[string]any{
		"url":       "https://example.com/edit",
		"title":     "Original",
		"expiresAt": expiresAt.Format(time.RFC3339Nano),
	})

	payload := func(title string, expiresAt *time.Time, isActive bool) map[string]any {
		body := map[string]any{
			"title":    title,
			"isActive": isActive,
		}
		if expiresAt == nil {
			body["expiresAt"] = nil
		} else {
			body["expiresAt"] = expiresAt.Format(time.RFC3339Nano)
		}
		return body
	}

	t.Run("rejects request without session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPut, "/urls/"+created.ID, payload("Hijacked", nil, false))
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects request without csrf token", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPut, "/urls/"+created.ID, payload("Hijacked", nil, false), filterCookie(cookies, "csrf_token")...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects malformed json", func(t *testing.T) {
		resp := doRequest(t, app, http.MethodPut, "/urls/"+created.ID, []byte("{"), cookies...)
		requireError(t, resp, http.StatusBadRequest, "invalid_json")
	})

	t.Run("rejects malformed expiresAt", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPut, "/urls/"+created.ID, map[string]any{
			"title":     "Original",
			"expiresAt": "not-a-timestamp",
			"isActive":  true,
		}, cookies...)
		requireError(t, resp, http.StatusBadRequest, "invalid_json")
	})

	t.Run("rejects title that is too long", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPut, "/urls/"+created.ID, payload(strings.Repeat("a", 257), nil, true), cookies...)
		requireError(t, resp, http.StatusUnprocessableEntity, "invalid_payload")
	})

	t.Run("returns not found for an invalid url id", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPut, "/urls/not-a-uuid", payload("Renamed", nil, true), cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")
	})

	t.Run("returns not found for an unknown url", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPut, "/urls/"+uuid.NewV7().String(), payload("Renamed", nil, true), cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")
	})

	t.Run("returns not found for another user's url", func(t *testing.T) {
		otherCookies := registerAndLogin(t, app, "edit_url_other_user")
		otherUrl := createUrl(t, app, otherCookies, map[string]any{
			"url":   "https://example.com/other-edit",
			"title": "Other",
		})

		resp := doJSON(t, app, http.MethodPut, "/urls/"+otherUrl.ID, payload("Hijacked", nil, false), cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")

		detail := getUrl(t, app, otherCookies, otherUrl.ID)
		require.Equal(t, "Other", detail.Title)
		require.True(t, detail.IsActive)
	})

	t.Run("updates title, expiry, and active status", func(t *testing.T) {
		newExpiresAt := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Millisecond)

		body := payload("  Renamed  ", &newExpiresAt, false)
		// Immutable fields must be ignored even when present.
		body["url"] = "https://evil.example/hijack"
		body["alias"] = "hijacked"
		body["userId"] = uuid.NewV7().String()

		detail := editUrl(t, app, cookies, created.ID, body)
		require.Equal(t, "Renamed", detail.Title)
		require.False(t, detail.IsActive)
		require.NotNil(t, detail.ExpiresAt)
		require.True(t, newExpiresAt.Equal(*detail.ExpiresAt))

		require.Equal(t, created.ID, detail.ID)
		require.Equal(t, userID, detail.UserID)
		require.Equal(t, created.URL, detail.URL)
		require.Equal(t, created.Alias, detail.Alias)
		require.WithinDuration(t, created.CreatedAt, detail.CreatedAt, time.Millisecond)

		persisted := getUrl(t, app, cookies, created.ID)
		require.Equal(t, "Renamed", persisted.Title)
		require.False(t, persisted.IsActive)
		require.Equal(t, created.URL, persisted.URL)
		require.Equal(t, created.Alias, persisted.Alias)
	})

	t.Run("clears the expiration time and reactivates the url", func(t *testing.T) {
		detail := editUrl(t, app, cookies, created.ID, payload("Renamed", nil, true))

		require.Equal(t, "Renamed", detail.Title)
		require.Nil(t, detail.ExpiresAt)
		require.True(t, detail.IsActive)

		persisted := getUrl(t, app, cookies, created.ID)
		require.Nil(t, persisted.ExpiresAt)
		require.True(t, persisted.IsActive)
	})

	t.Run("allows an empty title", func(t *testing.T) {
		detail := editUrl(t, app, cookies, created.ID, payload("   ", nil, true))
		require.Empty(t, detail.Title)

		persisted := getUrl(t, app, cookies, created.ID)
		require.Empty(t, persisted.Title)
	})
}

func TestDeleteUrl(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "delete_url_user")

	created := createUrl(t, app, cookies, map[string]any{
		"url":   "https://example.com/delete",
		"title": "Delete Me",
	})

	t.Run("rejects request without session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodDelete, "/urls/"+created.ID, nil)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects request without csrf token", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodDelete, "/urls/"+created.ID, nil, filterCookie(cookies, "csrf_token")...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("returns not found for an invalid url id", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodDelete, "/urls/not-a-uuid", nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")
	})

	t.Run("returns not found for an unknown url", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodDelete, "/urls/"+uuid.NewV7().String(), nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")
	})

	t.Run("returns not found for another user's url", func(t *testing.T) {
		otherCookies := registerAndLogin(t, app, "delete_url_other_user")
		otherUrl := createUrl(t, app, otherCookies, map[string]any{
			"url":   "https://example.com/other-delete",
			"title": "Other",
		})

		resp := doJSON(t, app, http.MethodDelete, "/urls/"+otherUrl.ID, nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")

		detail := getUrl(t, app, otherCookies, otherUrl.ID)
		require.Equal(t, "Other", detail.Title)
	})

	t.Run("soft deletes the url", func(t *testing.T) {
		before := listUrls(t, app, cookies, nil)
		require.Len(t, before.Items, 1)

		resp := doJSON(t, app, http.MethodDelete, "/urls/"+created.ID, nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		resp = doJSON(t, app, http.MethodGet, "/urls/"+created.ID, nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")

		after := listUrls(t, app, cookies, nil)
		require.Empty(t, after.Items)

		resp = doJSON(t, app, http.MethodDelete, "/urls/"+created.ID, nil, cookies...)
		requireError(t, resp, http.StatusNotFound, "url_not_found")
	})
}

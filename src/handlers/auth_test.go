package handlers_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	resp := doJSON(t, app, http.MethodPost, "/auth/register", map[string]string{
		"username":       "register_user",
		"name":           "Register User",
		"password":       "password123",
		"verifyPassword": "password123",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = doJSON(t, app, http.MethodPost, "/auth/register", map[string]string{
		"username":       "register_user",
		"name":           "Duplicate User",
		"password":       "password123",
		"verifyPassword": "password123",
	})
	requireError(t, resp, http.StatusConflict, "username_conflict")

	resp = doRequest(t, app, http.MethodPost, "/auth/register", []byte("{"))
	requireError(t, resp, http.StatusBadRequest, "invalid_json")

	resp = doJSON(t, app, http.MethodPost, "/auth/register", map[string]string{
		"username":       "missing_name",
		"password":       "password123",
		"verifyPassword": "password123",
	})
	requireError(t, resp, http.StatusUnprocessableEntity, "invalid_payload")

	statuses := doConcurrentJSON(t, app, http.MethodPost, "/auth/register", map[string]string{
		"username":       "race_register_user",
		"name":           "Race User",
		"password":       "password123",
		"verifyPassword": "password123",
	}, nil, nil)
	require.ElementsMatch(t, []int{http.StatusCreated, http.StatusConflict}, statuses)
}

func TestLogin(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	resp := doJSON(t, app, http.MethodPost, "/auth/register", map[string]string{
		"username":       "login_user",
		"name":           "Login User",
		"password":       "password123",
		"verifyPassword": "password123",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = doJSON(t, app, http.MethodPost, "/auth/login", map[string]string{
		"username": "login_user",
		"password": "password123",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	cookies := resp.Cookies()
	require.NotEmpty(t, cookieValue(cookies, "session_id"))
	require.NotEmpty(t, cookieValue(cookies, "session_token"))
	require.NotEmpty(t, cookieValue(cookies, "csrf_token"))

	resp = doJSON(t, app, http.MethodPost, "/auth/login", map[string]string{
		"username": "login_user",
		"password": "wrong-password",
	})
	requireError(t, resp, http.StatusUnauthorized, "unauthorized")
}

func TestHasSession(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "has_session_user")

	t.Run("allows a valid session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/auth/me", nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("rejects missing session cookies", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/auth/me", nil)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects a missing session id", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/auth/me", nil, filterCookie(cookies, "session_id")...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects a missing session token", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodGet, "/auth/me", nil, filterCookie(cookies, "session_token")...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects a malformed session id", func(t *testing.T) {
		invalidCookies := replaceCookie(cookies, "session_id", "not-a-uuid")
		resp := doJSON(t, app, http.MethodGet, "/auth/me", nil, invalidCookies...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects an unknown session", func(t *testing.T) {
		unknownCookies := replaceCookie(cookies, "session_id", uuid.NewV7().String())
		resp := doJSON(t, app, http.MethodGet, "/auth/me", nil, unknownCookies...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects a mismatched session token", func(t *testing.T) {
		mismatchedCookies := replaceCookie(cookies, "session_token", "not-the-session-token")
		resp := doJSON(t, app, http.MethodGet, "/auth/me", nil, mismatchedCookies...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects a logged out session", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPost, "/auth/logout", nil, cookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		resp = doJSON(t, app, http.MethodGet, "/auth/me", nil, cookies...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})
}

func TestHasCsrf(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "has_csrf_user")

	body := map[string]any{
		"url":   "https://example.com/has-csrf",
		"title": "Has CSRF",
	}

	t.Run("allows a matching csrf token", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPost, "/urls", body, cookies...)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("rejects a missing csrf cookie", func(t *testing.T) {
		resp := doJSON(t, app, http.MethodPost, "/urls", body, filterCookie(cookies, "csrf_token")...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects an empty csrf token", func(t *testing.T) {
		emptyCookies := replaceCookie(cookies, "csrf_token", "")
		resp := doJSON(t, app, http.MethodPost, "/urls", body, emptyCookies...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects a mismatched csrf token", func(t *testing.T) {
		mismatchedCookies := replaceCookie(cookies, "csrf_token", "not-the-csrf-token")
		resp := doJSON(t, app, http.MethodPost, "/urls", body, mismatchedCookies...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})

	t.Run("rejects another session's csrf token", func(t *testing.T) {
		otherCookies := registerAndLogin(t, app, "has_csrf_other_user")
		otherCsrf := cookieValue(otherCookies, "csrf_token")
		require.NotEmpty(t, otherCsrf)

		mismatchedCookies := replaceCookie(cookies, "csrf_token", otherCsrf)
		resp := doJSON(t, app, http.MethodPost, "/urls", body, mismatchedCookies...)
		requireError(t, resp, http.StatusUnauthorized, "unauthorized")
	})
}

func TestLogout(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "logout_user")

	resp := doJSON(t, app, http.MethodPost, "/auth/logout", nil, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doJSON(t, app, http.MethodGet, "/auth/me", nil, cookies...)
	requireError(t, resp, http.StatusUnauthorized, "unauthorized")
}

func TestEditUsername(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "edit_username_user")

	resp := doJSON(t, app, http.MethodPut, "/auth/usernames", map[string]string{
		"username": "renamed_user",
	}, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doJSON(t, app, http.MethodGet, "/auth/me", nil, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	account := decodeJSON[struct {
		Username string `json:"username"`
	}](t, resp)
	require.Equal(t, "renamed_user", account.Username)

	resp = doJSON(t, app, http.MethodPut, "/auth/usernames", map[string]string{
		"username": "renamed_user",
	}, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doJSON(t, app, http.MethodPost, "/auth/login", map[string]string{
		"username": "edit_username_user",
		"password": "password123",
	})
	requireError(t, resp, http.StatusUnauthorized, "unauthorized")

	resp = doJSON(t, app, http.MethodPost, "/auth/login", map[string]string{
		"username": "renamed_user",
		"password": "password123",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	registerAndLogin(t, app, "taken_username_user")

	resp = doJSON(t, app, http.MethodPut, "/auth/usernames", map[string]string{
		"username": "taken_username_user",
	}, cookies...)
	requireError(t, resp, http.StatusConflict, "username_conflict")

	retiredCookies := registerAndLogin(t, app, "retired_username_user")
	resp = doJSON(t, app, http.MethodDelete, "/auth/me", nil, retiredCookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doJSON(t, app, http.MethodPut, "/auth/usernames", map[string]string{
		"username": "retired_username_user",
	}, cookies...)
	requireError(t, resp, http.StatusConflict, "username_conflict")

	cookiesA := registerAndLogin(t, app, "race_rename_a")
	cookiesB := registerAndLogin(t, app, "race_rename_b")

	statuses := doConcurrentJSON(t, app, http.MethodPut, "/auth/usernames", map[string]string{
		"username": "race_rename_target",
	}, cookiesA, cookiesB)
	require.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, statuses)

	renamed := 0
	for _, accountCookies := range [][]*http.Cookie{cookiesA, cookiesB} {
		resp = doJSON(t, app, http.MethodGet, "/auth/me", nil, accountCookies...)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		account := decodeJSON[struct {
			Username string `json:"username"`
		}](t, resp)
		if account.Username == "race_rename_target" {
			renamed++
		}
	}
	require.Equal(t, 1, renamed)

	resp = doJSON(t, app, http.MethodPut, "/auth/usernames", map[string]string{
		"username": "no_csrf_user",
	}, filterCookie(cookies, "csrf_token")...)
	requireError(t, resp, http.StatusUnauthorized, "unauthorized")
}

func TestEditPassword(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "edit_password_user")

	resp := doJSON(t, app, http.MethodPut, "/auth/passwords", map[string]string{
		"oldPassword":       "password123",
		"newPassword":       "new-password123",
		"verifyNewPassword": "new-password123",
	}, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doJSON(t, app, http.MethodPost, "/auth/login", map[string]string{
		"username": "edit_password_user",
		"password": "password123",
	})
	requireError(t, resp, http.StatusUnauthorized, "unauthorized")

	resp = doJSON(t, app, http.MethodPost, "/auth/login", map[string]string{
		"username": "edit_password_user",
		"password": "new-password123",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGetAccount(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "account_user")

	resp := doJSON(t, app, http.MethodGet, "/auth/me", nil, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	account := decodeJSON[struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Username  string `json:"username"`
		CreatedAt string `json:"createdAt"`
	}](t, resp)
	require.NotEmpty(t, account.ID)
	require.Equal(t, "Test User", account.Name)
	require.Equal(t, "account_user", account.Username)
	require.NotEmpty(t, account.CreatedAt)

	resp = doJSON(t, app, http.MethodGet, "/auth/me", nil)
	requireError(t, resp, http.StatusUnauthorized, "unauthorized")
}

func TestDeleteAccount(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	cookies := registerAndLogin(t, app, "delete_user")

	resp := doJSON(t, app, http.MethodDelete, "/auth/me", nil, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doJSON(t, app, http.MethodGet, "/auth/me", nil, cookies...)
	requireError(t, resp, http.StatusUnauthorized, "unauthorized")

	resp = doJSON(t, app, http.MethodPost, "/auth/login", map[string]string{
		"username": "delete_user",
		"password": "password123",
	})
	requireError(t, resp, http.StatusUnauthorized, "unauthorized")

	resp = doJSON(t, app, http.MethodPost, "/auth/register", map[string]string{
		"username":       "delete_user",
		"name":           "Reused User",
		"password":       "password123",
		"verifyPassword": "password123",
	})
	requireError(t, resp, http.StatusConflict, "username_conflict")
}

func TestDeleteAccountSoftDeletesUrls(t *testing.T) {
	t.Parallel()
	app, dbURL, _ := setupTestAppWithDB(t)
	cookies := registerAndLogin(t, app, "delete_user_with_urls")

	created := createUrl(t, app, cookies, map[string]any{
		"url":   "https://example.com/delete-with-account",
		"title": "Delete With Account",
	})

	otherCookies := registerAndLogin(t, app, "delete_user_with_urls_other")
	otherUrl := createUrl(t, app, otherCookies, map[string]any{
		"url":   "https://example.com/keep",
		"title": "Keep",
	})

	resp := doJSON(t, app, http.MethodDelete, "/auth/me", nil, cookies...)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	require.Eventually(t, func() bool {
		var deleted int
		err := db.QueryRow(
			`select count(*) from shortening.urls where id = $1 and deleted_at is not null`,
			created.ID,
		).Scan(&deleted)
		return err == nil && deleted == 1
	}, 5*time.Second, 50*time.Millisecond)

	var otherActive int
	require.NoError(t, db.QueryRow(
		`select count(*) from shortening.urls where id = $1 and deleted_at is null`,
		otherUrl.ID,
	).Scan(&otherActive))
	require.Equal(t, 1, otherActive)
}

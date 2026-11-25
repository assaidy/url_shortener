package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/assaidy/moon"
	"github.com/stretchr/testify/require"
)

func doRequest(t *testing.T, app *moon.App, method, path string, body []byte, cookies ...*http.Cookie) *http.Response {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp := app.Test(req)
	t.Cleanup(func() {
		_ = resp.Body.Close()
	})
	return resp
}

func doJSON(t *testing.T, app *moon.App, method, path string, body any, cookies ...*http.Cookie) *http.Response {
	t.Helper()

	raw, err := json.Marshal(body)
	require.NoError(t, err)
	return doRequest(t, app, method, path, raw, cookies...)
}

// doConcurrentJSON issues one request per cookie set concurrently and returns
// the response statuses in completion order.
func doConcurrentJSON(t *testing.T, app *moon.App, method, path string, body any, cookieSets ...[]*http.Cookie) []int {
	t.Helper()

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	statuses := make(chan int, len(cookieSets))
	var wg sync.WaitGroup
	for _, cookies := range cookieSets {
		wg.Add(1)
		go func(cookies []*http.Cookie) {
			defer wg.Done()

			req := httptest.NewRequest(method, path, bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}

			resp := app.Test(req)
			defer resp.Body.Close()

			statuses <- resp.StatusCode
		}(cookies)
	}
	wg.Wait()
	close(statuses)

	result := make([]int, 0, len(cookieSets))
	for status := range statuses {
		result = append(result, status)
	}
	return result
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()

	var value T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&value))
	return value
}

type apiError struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

func requireError(t *testing.T, resp *http.Response, status int, kind string) {
	t.Helper()

	require.Equal(t, status, resp.StatusCode)
	body := decodeJSON[apiError](t, resp)
	require.Equal(t, kind, body.Kind)
}

func registerAndLogin(t *testing.T, app *moon.App, username string) []*http.Cookie {
	t.Helper()

	resp := doJSON(t, app, http.MethodPost, "/auth/register", map[string]string{
		"username":       username,
		"name":           "Test User",
		"password":       "password123",
		"verifyPassword": "password123",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp = doJSON(t, app, http.MethodPost, "/auth/login", map[string]string{
		"username": username,
		"password": "password123",
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	return resp.Cookies()
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func filterCookie(cookies []*http.Cookie, name string) []*http.Cookie {
	filtered := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != name {
			filtered = append(filtered, cookie)
		}
	}
	return filtered
}

// replaceCookie returns a copy of cookies with the named cookie's value
// replaced.
func replaceCookie(cookies []*http.Cookie, name, value string) []*http.Cookie {
	replaced := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name == name {
			copy := *cookie
			copy.Value = value
			cookie = &copy
		}
		replaced = append(replaced, cookie)
	}
	return replaced
}

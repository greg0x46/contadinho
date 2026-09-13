package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"contadinho-go/internal/auth"
	"contadinho-go/internal/httpapi"
	"contadinho-go/internal/settings"
)

func TestBrowserAuthentication(t *testing.T) {
	srv, conn := newLockedTestServer(t)
	request := func(method, path, body, origin, csrf string, cookie *http.Cookie) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		req.Header.Set("Origin", origin)
		req.Header.Set("X-Contadinho-Request", csrf)
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}
	assert := func(resp *http.Response, want int) {
		t.Helper()
		if resp.StatusCode != want {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d want %d: %s", resp.StatusCode, want, body)
		}
	}
	payload := `{"email":"owner@example.com","password":"` + testPassword + `"}`
	for _, path := range []string{"/api/categories", "/api/preferences", "/api/sync-runs", "/api/unknown", "/api/setup/status"} {
		assert(request("GET", path, "", "", "", nil), 401)
	}
	for _, origin := range []string{"", "https://evil.example"} {
		assert(request("POST", "/api/auth/login", payload, origin, "1", nil), 403)
	}
	assert(request("POST", "/api/auth/login", payload, testOrigin, "", nil), 403)
	assert(request("POST", "/api/auth/login", payload+" trailing", testOrigin, "1", nil), 422)
	assert(request("POST", "/api/auth/login", `{"email":"owner@example.com","password":"wrong"}`, testOrigin, "1", nil), 401)
	resp := request("POST", "/api/auth/login", payload, testOrigin, "1", nil)
	assert(resp, 200)
	cookie := resp.Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Fatalf("cookie: %+v", cookie)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
	assert(request("GET", "/api/categories", "", "", "", cookie), 200)
	assert(request("GET", "/api/categories", "", "", "", nil), 401)
	resp = request("POST", "/api/auth/login", payload, testOrigin, "1", nil)
	assert(resp, 200)
	second := resp.Cookies()[0]
	assert(request("POST", "/api/auth/logout", "", testOrigin, "1", cookie), 200)
	assert(request("GET", "/api/categories", "", "", "", cookie), 401)
	assert(request("GET", "/api/categories", "", "", "", second), 200)
	for _, path := range []string{"/api/setup", "/api/unlock"} {
		assert(request("POST", path, payload, testOrigin, "1", second), 404)
	}
	assert(request("PUT", "/api/settings/pluggy", `{"pluggy_client_id":"id","pluggy_client_secret":"secret","pluggy_item_id":"item"}`, testOrigin, "1", second), 200)
	master := bytes.Repeat([]byte{42}, 32)
	if got, _, err := settings.Get(context.Background(), conn, "pluggy.client_secret", master); err != nil || got != "secret" {
		t.Fatalf("credentials: %q %v", got, err)
	}
	assert(request("PUT", "/api/auth/password", `{"current_password":"`+testPassword+`","password":"a completely different password"}`, testOrigin, "1", second), 200)
	assert(request("GET", "/api/categories", "", "", "", second), 401)
}
func TestAuthLimitsAndDatabaseFailure(t *testing.T) {
	srv, conn := newLockedTestServer(t)
	for i := 0; i < 6; i++ {
		resp := doJSON(t, "POST", srv.URL+"/api/auth/login", map[string]string{"email": "owner@example.com", "password": "incorrect"})
		resp.Body.Close()
		want := 401
		if i == 5 {
			want = 429
		}
		if resp.StatusCode != want {
			t.Fatalf("attempt %d: %d", i, resp.StatusCode)
		}
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	resp := doJSON(t, "GET", srv.URL+"/api/auth/session", nil)
	// Without a cookie the public endpoint does not need the DB and leaks no state.
	var state map[string]any
	decodeJSON(t, resp, &state)
	if state["authenticated"] != false {
		t.Fatal(state)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/categories", nil)
	req.AddCookie(&http.Cookie{Name: "contadinho_session", Value: strings.Repeat("A", 43)})
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatal(resp.StatusCode)
	}
}
func TestProductionCookieAndServerRestart(t *testing.T) {
	_, conn, keys := newTestServerWithSession(t)
	handler := httpapi.NewServer(conn, fstest.MapFS{"index.html": {Data: []byte("spa")}}, keys, auth.Config{PublicURL: "https://finance.example"})
	req := httptest.NewRequest("POST", "https://finance.example/api/auth/login", strings.NewReader(`{"email":"owner@example.com","password":"`+testPassword+`"}`))
	req.Header.Set("Origin", "https://finance.example")
	req.Header.Set("X-Contadinho-Request", "1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	cookie := response.Result().Cookies()[0]
	if !cookie.Secure || cookie.Name != "__Host-contadinho_session" {
		t.Fatal(cookie)
	}
	// Recreate both encryption holder and HTTP server: browser auth is in the DB.
	keys = settings.NewSecrets(bytes.Repeat([]byte{42}, 32))
	handler = httpapi.NewServer(conn, fstest.MapFS{}, keys, auth.Config{PublicURL: "https://finance.example"})
	req = httptest.NewRequest("GET", "https://finance.example/api/auth/session", nil)
	req.AddCookie(cookie)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	var state map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state["authenticated"] != true {
		t.Fatal(state)
	}
}

func TestLoginGlobalLimitBoundsDistinctEmails(t *testing.T) {
	srv, _ := newLockedTestServer(t)
	for i := 0; i < 31; i++ {
		resp := doJSON(t, "POST", srv.URL+"/api/auth/login", map[string]string{"email": fmt.Sprintf("unknown%d@example.com", i), "password": testPassword})
		resp.Body.Close()
		want := 401
		if i == 30 {
			want = 429
		}
		if resp.StatusCode != want {
			t.Fatalf("attempt %d: %d", i, resp.StatusCode)
		}
	}
}

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nebulaiq/internal/config"
	"nebulaiq/internal/testutil"
	"nebulaiq/internal/workflow"
)

func TestGuestOwnershipAndCSRFBoundary(t *testing.T) {
	s := testutil.Database(t)
	c := config.Config{Local: true, OwnerQuota: 20, GlobalQuota: 100, RunTimeout: time.Minute}
	e := workflow.New(context.Background(), s, nil, nil, c)
	server := httptest.NewServer(New(s, e, c))
	defer server.Close()
	request := func(method, path, payload string, cookie *http.Cookie, origin string, header bool) *http.Response {
		t.Helper()
		r, _ := http.NewRequest(method, server.URL+path, bytes.NewBufferString(payload))
		r.Header.Set("Content-Type", "application/json")
		if header {
			r.Header.Set("X-Nebula-Request", "1")
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		res, err := server.Client().Do(r)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	bad := request("POST", "/api/access", `{"code":""}`, nil, "", false)
	if bad.StatusCode != 403 {
		t.Fatal("missing CSRF header accepted")
	}
	bad.Body.Close()
	access := request("POST", "/api/access", `{"code":""}`, nil, "", true)
	if access.StatusCode != 200 || len(access.Cookies()) == 0 {
		t.Fatal("guest session failed")
	}
	cookie := access.Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("cookie flags missing")
	}
	access.Body.Close()
	res := request("POST", "/api/conversations", `{}`, cookie, "", true)
	if res.StatusCode != 201 {
		t.Fatal("conversation create failed")
	}
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()
	foreign := request("POST", "/api/access", `{"code":""}`, nil, "", true)
	other := foreign.Cookies()[0]
	foreign.Body.Close()
	res = request("GET", "/api/conversations/"+chat.ID, "", other, "", true)
	if res.StatusCode != 404 {
		t.Fatal("foreign owner read private history")
	}
	res.Body.Close()
	res = request("POST", "/api/conversations", `{}`, cookie, "https://attacker.example", true)
	if res.StatusCode != 403 {
		t.Fatal("cross-origin mutation accepted")
	}
	res.Body.Close()
	res = request("GET", "/.env", "", cookie, "", true)
	if res.StatusCode != 404 {
		t.Fatal("embedded file server exposed env")
	}
	res.Body.Close()
	res = request("DELETE", "/api/access", `{}`, cookie, "", true)
	if res.StatusCode != 200 {
		t.Fatal("logout failed")
	}
	res.Body.Close()
	res = request("GET", "/api/conversations", "", cookie, "", true)
	if res.StatusCode != 401 {
		t.Fatal("revoked token still authorized")
	}
	res.Body.Close()
}

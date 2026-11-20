package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"englishlisten/sdwan/internal/app"
	"englishlisten/sdwan/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func TestPlatformSessionBoundary(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("operator-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewRouter(app.NewService(nil, config.Config{PlatformEmail: "operator@example.test", PlatformPasswordHash: string(hash)}), nil)
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}
	for _, path := range []string{"/api/platform/tenants", "/api/platform/tenants/example", "/api/platform/health"} {
		if got := call(http.MethodGet, path, "sdwan_admin_example", "").Code; got != http.StatusUnauthorized {
			t.Fatalf("%s: enterprise token got %d", path, got)
		}
	}
	login := call(http.MethodPost, "/api/platform/login", "", `{"email":"operator@example.test","password":"operator-password"}`)
	var session struct{ Token string }
	if login.Code != http.StatusOK || json.Unmarshal(login.Body.Bytes(), &session) != nil || session.Token == "" {
		t.Fatalf("operator login failed: %d", login.Code)
	}
	if got := call(http.MethodPost, "/api/platform/logout", session.Token, "").Code; got != http.StatusNoContent {
		t.Fatalf("logout got %d", got)
	}
	if got := call(http.MethodGet, "/api/platform/tenants", session.Token, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("revoked session got %d", got)
	}
}

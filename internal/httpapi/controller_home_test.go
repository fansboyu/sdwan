package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestControllerHomeExplainsAPIAndConsole(t *testing.T) {
	handler := NewRouter(nil, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("content type = %q, want HTML", contentType)
	}
	for _, text := range []string{"Controller API 正在运行", "http://localhost:8082", "/healthz", "/readyz"} {
		if !strings.Contains(recorder.Body.String(), text) {
			t.Errorf("response missing %q", text)
		}
	}
}

func TestControllerHomeDoesNotCatchOtherPaths(t *testing.T) {
	handler := NewRouter(nil, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

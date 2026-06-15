package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthRouteReturnsOK(t *testing.T) {
	router := NewRouter(Dependencies{})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Equal(bytes.TrimSpace(recorder.Body.Bytes()), []byte(`{"ok":true}`)) {
		t.Fatalf("health body = %s, want {\"ok\":true}", recorder.Body.String())
	}
}

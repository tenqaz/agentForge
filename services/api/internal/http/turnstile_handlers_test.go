package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentforge.local/services/api/internal/turnstile"
	"github.com/gin-gonic/gin"
)

func newTurnstileTestRouter(svc *turnstile.Service) *gin.Engine {
	r := gin.New()
	NewTurnstileHandlers(svc).Register(r.Group("/api"))
	return r
}

func TestTurnstileConfigDisabled(t *testing.T) {
	svc := turnstile.NewVerifier("", "", "", "", nil)
	router := newTurnstileTestRouter(svc)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/turnstile/config", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var resp struct {
		Sitekey string `json:"sitekey"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Enabled {
		t.Fatalf("enabled = true, want false")
	}
	if resp.Sitekey != "" {
		t.Fatalf("sitekey = %q, want empty", resp.Sitekey)
	}
}

func TestTurnstileConfigEnabled(t *testing.T) {
	svc := turnstile.NewVerifier("sec", "site", "", "", nil)
	router := newTurnstileTestRouter(svc)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/turnstile/config", nil))

	var resp struct {
		Sitekey string `json:"sitekey"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Enabled {
		t.Fatalf("enabled = false, want true")
	}
	if resp.Sitekey != "site" {
		t.Fatalf("sitekey = %q, want site", resp.Sitekey)
	}
}

func TestRequireTurnstileDisabledAllows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	if !requireTurnstile(c, turnstile.NewVerifier("", "", "", "", nil), "tok", "login") {
		t.Fatalf("requireTurnstile = false, want true when disabled")
	}
}

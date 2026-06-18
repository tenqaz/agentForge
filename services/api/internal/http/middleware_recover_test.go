package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoverMiddlewareReturnsInternalErrorResponse(t *testing.T) {
	router := gin.New()
	router.Use(RequestIDMiddleware(), RecoverMiddleware())
	router.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal panic response: %v", err)
	}
	if response["message"] != "internal error" {
		t.Fatalf("message = %q, want internal error", response["message"])
	}
	if response["requestId"] == "" {
		t.Fatalf("requestId missing: %s", recorder.Body.String())
	}
}

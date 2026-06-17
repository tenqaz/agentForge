package http

import (
	"net/http"

	"github.com/google/uuid"
)

// RequestIDMiddleware 为每个请求添加唯一 ID
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

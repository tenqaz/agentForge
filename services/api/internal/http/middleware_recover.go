package http

import "net/http"

func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				recoverPanic(w, r, recovered)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

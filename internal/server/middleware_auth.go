package server

import (
	"net/http"
	"strings"
)

func AuthMiddleware(fixedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeOpenAIError(w, http.StatusUnauthorized, "invalid_api_key", "missing or invalid Authorization header")
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if token == "" || token != fixedKey {
				writeOpenAIError(w, http.StatusUnauthorized, "invalid_api_key", "invalid api key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

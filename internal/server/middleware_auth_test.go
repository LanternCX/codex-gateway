package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mw := AuthMiddleware("fixed-key")
	h := mw(next)

	t.Run("missing header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		res := httptest.NewRecorder()

		h.ServeHTTP(res, req)

		if res.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", res.Code)
		}

		var out errorEnvelope
		if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
			t.Fatalf("parse error body: %v", err)
		}

		if out.Error.Code != "invalid_api_key" {
			t.Fatalf("unexpected error code: %q", out.Error.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		res := httptest.NewRecorder()

		h.ServeHTTP(res, req)

		if res.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", res.Code)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer fixed-key")
		res := httptest.NewRecorder()

		h.ServeHTTP(res, req)

		if res.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", res.Code)
		}
	})
}

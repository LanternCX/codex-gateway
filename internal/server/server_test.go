package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codex-gateway/internal/upstream"
)

func TestServerRoutes_Healthz(t *testing.T) {
	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient("https://example.com", 0),
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestServerRoutes_ProtectedRouteRequiresAuth(t *testing.T) {
	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient("https://example.com", 0),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func TestServerRoutes_LogsRequests(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient("https://example.com", 0),
		Logger:         logger,
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	out := buf.String()
	if !strings.Contains(out, `"msg":"request completed"`) {
		t.Fatalf("expected request log, got: %s", out)
	}

	if !strings.Contains(out, `"path":"/healthz"`) {
		t.Fatalf("expected path in log, got: %s", out)
	}

	if !strings.Contains(out, `"status":200`) {
		t.Fatalf("expected status in log, got: %s", out)
	}
}

type nopTokenProvider struct{}

func (nopTokenProvider) AccessToken(ctx context.Context) (string, error)  { return "", nil }
func (nopTokenProvider) ForceRefresh(ctx context.Context) (string, error) { return "", nil }

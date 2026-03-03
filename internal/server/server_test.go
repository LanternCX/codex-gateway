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

func TestServerRoutes_ResponsesRouteRequiresAuth(t *testing.T) {
	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient("https://example.com", 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func TestServerRoutes_ResponsesRouteRejectsGet(t *testing.T) {
	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient("https://example.com", 0),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	req.Header.Set("Authorization", "Bearer fixed-key")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", res.Code)
	}

	if got := res.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("expected Allow header %q, got %q", http.MethodPost, got)
	}
}

func TestServerRoutes_ResponsesRouteAuthenticatedPostProxies(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req.Header.Set("Authorization", "Bearer fixed-key")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res.Code)
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

func TestServerRoutes_GeneratesRequestIDWhenMissing(t *testing.T) {
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

	requestID := res.Header().Get("X-Request-ID")
	if strings.TrimSpace(requestID) == "" {
		t.Fatal("expected response header X-Request-ID")
	}

	out := buf.String()
	if !strings.Contains(out, `"request_id":"`+requestID+`"`) {
		t.Fatalf("expected request_id in log, got: %s", out)
	}
}

func TestServerRoutes_PreservesIncomingRequestID(t *testing.T) {
	const incomingRequestID = "req-from-client-001"
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient("https://example.com", 0),
		Logger:         logger,
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", incomingRequestID)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if got := res.Header().Get("X-Request-ID"); got != incomingRequestID {
		t.Fatalf("expected request id %q, got %q", incomingRequestID, got)
	}

	out := buf.String()
	if !strings.Contains(out, `"request_id":"`+incomingRequestID+`"`) {
		t.Fatalf("expected preserved request_id in log, got: %s", out)
	}
}

type nopTokenProvider struct{}

func (nopTokenProvider) AccessToken(ctx context.Context) (string, error)  { return "", nil }
func (nopTokenProvider) ForceRefresh(ctx context.Context) (string, error) { return "", nil }

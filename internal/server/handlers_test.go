package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codex-gateway/internal/auth"
	"codex-gateway/internal/upstream"
)

func TestProxyModels(t *testing.T) {
	upstreamCalled := false
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
			t.Fatalf("unexpected upstream auth: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer fixed-key")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if !upstreamCalled {
		t.Fatal("expected upstream call")
	}

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	if !strings.Contains(res.Body.String(), `"object":"list"`) {
		t.Fatalf("unexpected response body: %s", res.Body.String())
	}
}

func TestProxyChatCompletionsStreamPassThrough(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		if !strings.Contains(string(body), `"stream":true`) {
			t.Fatalf("unexpected body: %s", string(body))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"id\":\"1\"}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x","messages":[],"stream":true}`))
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("unexpected content-type: %q", ct)
	}

	if !strings.Contains(res.Body.String(), "[DONE]") {
		t.Fatalf("unexpected stream body: %s", res.Body.String())
	}
}

func TestProxyTokenProviderFailure(t *testing.T) {
	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{err: errors.New("no token")},
		UpstreamClient: upstream.NewClient("https://example.com", 0),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer fixed-key")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", res.Code)
	}
}

func TestProxyModels_ForwardsRequestIDToUpstream(t *testing.T) {
	const requestID = "req-forward-001"

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request-ID"); got != requestID {
			t.Fatalf("expected X-Request-ID %q, got %q", requestID, got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("X-Request-ID", requestID)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

type staticTokenProvider struct {
	token string
	err   error
}

func (s staticTokenProvider) AccessToken(ctx context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.token, nil
}

func (s staticTokenProvider) ForceRefresh(ctx context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.token, nil
}

var _ TokenProvider = staticTokenProvider{}
var _ = auth.ErrNotLoggedIn

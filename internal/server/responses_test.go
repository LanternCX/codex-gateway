package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codex-gateway/internal/upstream"
)

func TestResponses_CodexCompatProxiesToCodexResponsesPathWithOriginatorAndSSEPassThrough(t *testing.T) {
	const reqBody = `{"model":"gpt-5.3-codex","instructions":"You are a helpful assistant.","input":[{"role":"user","content":"hello"}],"stream":true}`
	const reqContentType = "application/json; charset=utf-8"
	const upstreamSSE = "data: {\"id\":\"resp_1\"}\n\ndata: [DONE]\n\n"

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if got := r.Header.Get("originator"); got != "opencode" {
			t.Fatalf("unexpected originator header: %q", got)
		}

		if got := r.Header.Get("Content-Type"); got != reqContentType {
			t.Fatalf("unexpected content type: %q", got)
		}

		if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		if string(body) != reqBody {
			t.Fatalf("unexpected upstream body: %s", string(body))
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(upstreamSSE))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:        "fixed-key",
		CodexCompat:        true,
		ResponsesPath:      "/v1/responses",
		CodexResponsesPath: "/backend-api/codex/responses",
		CodexOriginator:    "opencode",
		TokenProvider:      staticTokenProvider{token: "oauth-token"},
		UpstreamClient:     upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("Content-Type", reqContentType)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("unexpected response content type: %q", ct)
	}

	if got := res.Body.String(); got != upstreamSSE {
		t.Fatalf("unexpected response body: %q", got)
	}
}

func TestResponses_CodexCompatAddsDefaultInstructionsWhenMissing(t *testing.T) {
	const reqBody = `{"model":"gpt-5.3-codex","input":[{"role":"user","content":"hello"}],"stream":true}`
	const reqContentType = "application/json"
	const upstreamSSE = "data: {\"id\":\"resp_2\"}\n\ndata: [DONE]\n\n"

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("parse request body: %v", err)
		}

		if got := strings.TrimSpace(asString(payload["instructions"])); got != "You are a helpful assistant." {
			t.Fatalf("expected default instructions, got %q", got)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(upstreamSSE))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:        "fixed-key",
		CodexCompat:        true,
		ResponsesPath:      "/v1/responses",
		CodexResponsesPath: "/backend-api/codex/responses",
		CodexOriginator:    "opencode",
		TokenProvider:      staticTokenProvider{token: "oauth-token"},
		UpstreamClient:     upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("Content-Type", reqContentType)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("unexpected response content type: %q", ct)
	}

	if got := res.Body.String(); got != upstreamSSE {
		t.Fatalf("unexpected response body: %q", got)
	}
}

func TestResponses_CodexCompatOmitsMaxOutputTokensWhenForwarding(t *testing.T) {
	const reqBody = `{"model":"gpt-5.3-codex","instructions":"You are a helpful assistant.","input":[{"role":"user","content":"hello"}],"max_output_tokens":1024,"stream":true}`
	const reqContentType = "application/json"
	const upstreamSSE = "data: {\"id\":\"resp_3\"}\n\ndata: [DONE]\n\n"

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("parse request body: %v", err)
		}

		if _, ok := payload["max_output_tokens"]; ok {
			t.Fatalf("expected max_output_tokens to be omitted, got %v", payload["max_output_tokens"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(upstreamSSE))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:        "fixed-key",
		CodexCompat:        true,
		ResponsesPath:      "/v1/responses",
		CodexResponsesPath: "/backend-api/codex/responses",
		CodexOriginator:    "opencode",
		TokenProvider:      staticTokenProvider{token: "oauth-token"},
		UpstreamClient:     upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("Content-Type", reqContentType)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("unexpected response content type: %q", ct)
	}

	if got := res.Body.String(); got != upstreamSSE {
		t.Fatalf("unexpected response body: %q", got)
	}
}

func TestResponses_OpenAIAPIProxiesToResponsesPathAndReturnsJSON(t *testing.T) {
	const reqBody = `{"model":"gpt-5.3","input":"hello"}`
	const reqContentType = "application/json"
	const upstreamJSON = `{"id":"resp_123","object":"response"}`

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/upstream/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if got := r.Header.Get("originator"); got != "" {
			t.Fatalf("expected no originator header, got %q", got)
		}

		if got := r.Header.Get("Content-Type"); got != reqContentType {
			t.Fatalf("unexpected content type: %q", got)
		}

		if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		if string(body) != reqBody {
			t.Fatalf("unexpected upstream body: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(upstreamJSON))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		CodexCompat:    false,
		ResponsesPath:  "/upstream/v1/responses",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/upstream/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("Content-Type", reqContentType)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unexpected response content type: %q", ct)
	}

	if got := strings.TrimSpace(res.Body.String()); got != upstreamJSON {
		t.Fatalf("unexpected response body: %s", got)
	}
}

func TestResponses_InvalidJSONReturns400WithoutUpstreamCall(t *testing.T) {
	upstreamCalled := false
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_123"}`))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		CodexCompat:    false,
		ResponsesPath:  "/upstream/v1/responses",
		TokenProvider:  staticTokenProvider{token: "oauth-token"},
		UpstreamClient: upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/upstream/v1/responses", strings.NewReader(`{"model":"gpt-5.3","input":"hello"`))
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if upstreamCalled {
		t.Fatal("expected gateway to reject invalid JSON before upstream call")
	}

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", res.Code, res.Body.String())
	}

	if !strings.Contains(res.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("expected invalid_request error code, got body=%s", res.Body.String())
	}
}

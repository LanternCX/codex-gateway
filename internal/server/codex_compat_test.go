package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codex-gateway/internal/upstream"
)

func TestCodexCompat_ModelsReturnsStaticList(t *testing.T) {
	h := New(Dependencies{
		FixedAPIKey:    "fixed-key",
		CodexCompat:    true,
		TokenProvider:  staticTokenProvider{token: "token"},
		UpstreamClient: upstream.NewClient("https://example.com", 0),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer fixed-key")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	if !strings.Contains(res.Body.String(), "gpt-5.3-codex") {
		t.Fatalf("expected codex model list, got: %s", res.Body.String())
	}
}

func TestCodexCompat_ChatCompletionsNonStream(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if got := r.Header.Get("originator"); got != "opencode" {
			t.Fatalf("unexpected originator: %q", got)
		}

		if got := r.Header.Get("ChatGPT-Account-Id"); got != "acct-123" {
			t.Fatalf("unexpected account header: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("parse request body: %v", err)
		}

		if req["stream"] != true {
			t.Fatalf("expected stream=true, got %v", req["stream"])
		}
		if req["store"] != false {
			t.Fatalf("expected store=false, got %v", req["store"])
		}
		if strings.TrimSpace(req["instructions"].(string)) == "" {
			t.Fatal("expected non-empty instructions")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: response.created\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-5.3-codex\"}}\n\n"))
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"delta\":\"hello\"}\n\n"))
		_, _ = w.Write([]byte("event: response.completed\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_1\",\"created_at\":1700000000,\"model\":\"gpt-5.3-codex\",\"usage\":{\"input_tokens\":11,\"output_tokens\":3,\"total_tokens\":14}}}\n\n"))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:        "fixed-key",
		CodexCompat:        true,
		CodexResponsesPath: "/backend-api/codex/responses",
		CodexOriginator:    "opencode",
		TokenProvider:      staticTokenProvider{token: fakeTokenWithAccount("acct-123")},
		UpstreamClient:     upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.3-codex","messages":[{"role":"user","content":"hello"}],"stream":false}`))
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	if !strings.Contains(res.Body.String(), `"object":"chat.completion"`) {
		t.Fatalf("unexpected response body: %s", res.Body.String())
	}

	if !strings.Contains(res.Body.String(), `"content":"hello"`) {
		t.Fatalf("expected hello content, got: %s", res.Body.String())
	}
}

func TestCodexCompat_ChatCompletionsStream(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: response.created\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_2\",\"created_at\":1700000001,\"model\":\"gpt-5.3-codex\"}}\n\n"))
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"delta\":\"he\"}\n\n"))
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"delta\":\"llo\"}\n\n"))
		_, _ = w.Write([]byte("event: response.completed\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_2\",\"created_at\":1700000001,\"model\":\"gpt-5.3-codex\"}}\n\n"))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:        "fixed-key",
		CodexCompat:        true,
		CodexResponsesPath: "/backend-api/codex/responses",
		TokenProvider:      staticTokenProvider{token: fakeTokenWithAccount("acct-123")},
		UpstreamClient:     upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.3-codex","messages":[{"role":"user","content":"hello"}],"stream":true}`))
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	if ct := res.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", ct)
	}

	body := res.Body.String()
	if !strings.Contains(body, "chat.completion.chunk") {
		t.Fatalf("expected chat completion chunk payloads, got: %s", body)
	}
	if !strings.Contains(body, "[DONE]") {
		t.Fatalf("expected [DONE], got: %s", body)
	}
}

func TestToCodexResponsesRequest_IgnoresMaxTokensForCodexBackend(t *testing.T) {
	maxTokens := 256
	converted, err := toCodexResponsesRequest(chatCompletionRequest{
		Model: "gpt-5.3-codex",
		Messages: []chatMessage{
			{Role: "user", Content: "hello"},
		},
		Stream:    false,
		MaxTokens: &maxTokens,
	})
	if err != nil {
		t.Fatalf("convert request: %v", err)
	}

	encoded, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("marshal converted request: %v", err)
	}

	if strings.Contains(string(encoded), "max_output_tokens") {
		t.Fatalf("expected max_output_tokens to be omitted, got: %s", string(encoded))
	}
}

func fakeTokenWithAccount(accountID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"` + accountID + `"}}`))
	return header + "." + payload + ".sig"
}

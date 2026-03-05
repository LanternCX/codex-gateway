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

	if strings.Contains(string(encoded), `"max_output_tokens"`) {
		t.Fatalf("expected max_output_tokens to be omitted, got: %s", string(encoded))
	}
}

func TestToCodexResponsesRequest_MapsToolCallingFieldsForCodexBackend(t *testing.T) {
	parallelToolCalls := true
	maxTokens := 256
	converted, err := toCodexResponsesRequest(chatCompletionRequest{
		Model: "gpt-5.3-codex",
		Messages: []chatMessage{
			{Role: "system", Content: "You are a tool-first assistant."},
			{Role: "assistant", ToolCalls: []chatToolCall{{
				ID:   "call_lookup_1",
				Type: "function",
				Function: chatFunctionCall{
					Name:      "lookup_weather",
					Arguments: `{"city":"SF"}`,
				},
			}}},
			{Role: "tool", ToolCallID: "call_lookup_1", Content: `{"temp_c":18}`},
			{Role: "user", Content: "Summarize result."},
		},
		Tools: []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "lookup_weather",
					"description": "lookup weather by city",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"city": map[string]any{"type": "string"},
						},
						"required": []any{"city"},
					},
					"strict": true,
				},
			},
		},
		ToolChoice: map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": "lookup_weather",
			},
		},
		ParallelToolCalls: &parallelToolCalls,
		ReasoningEffort:   "high",
		MaxTokens:         &maxTokens,
	})
	if err != nil {
		t.Fatalf("convert request: %v", err)
	}

	encoded, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("marshal converted request: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode converted request: %v", err)
	}

	if _, ok := payload["max_output_tokens"]; ok {
		t.Fatalf("expected max_output_tokens to be omitted, got %v", payload["max_output_tokens"])
	}

	reasoning, ok := payload["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("expected reasoning object, got %T", payload["reasoning"])
	}
	if got := reasoning["effort"]; got != "high" {
		t.Fatalf("expected reasoning effort high, got %v", got)
	}

	if got, ok := payload["parallel_tool_calls"].(bool); !ok || !got {
		t.Fatalf("expected parallel_tool_calls=true, got %v", payload["parallel_tool_calls"])
	}

	toolChoice, ok := payload["tool_choice"].(map[string]any)
	if !ok {
		t.Fatalf("expected tool_choice object, got %T", payload["tool_choice"])
	}
	if got := toolChoice["type"]; got != "function" {
		t.Fatalf("expected tool_choice.type=function, got %v", got)
	}
	if got := toolChoice["name"]; got != "lookup_weather" {
		t.Fatalf("expected tool_choice.name=lookup_weather, got %v", got)
	}

	tools, ok := payload["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected one converted tool, got %v", payload["tools"])
	}
	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tool object, got %T", tools[0])
	}
	if got := tool["type"]; got != "function" {
		t.Fatalf("expected tool type=function, got %v", got)
	}
	if got := tool["name"]; got != "lookup_weather" {
		t.Fatalf("expected tool name=lookup_weather, got %v", got)
	}

	input, ok := payload["input"].([]any)
	if !ok {
		t.Fatalf("expected input array, got %T", payload["input"])
	}

	foundFunctionCall := false
	foundFunctionCallOutput := false
	for _, raw := range input {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch item["type"] {
		case "function_call":
			if item["call_id"] == "call_lookup_1" && item["name"] == "lookup_weather" {
				foundFunctionCall = true
			}
		case "function_call_output":
			if item["call_id"] == "call_lookup_1" {
				foundFunctionCallOutput = true
			}
		}
	}

	if !foundFunctionCall {
		t.Fatalf("expected function_call item in input, got %v", input)
	}
	if !foundFunctionCallOutput {
		t.Fatalf("expected function_call_output item in input, got %v", input)
	}
}

func TestToCodexResponsesRequest_RejectsToolMessageWithoutToolCallID(t *testing.T) {
	_, err := toCodexResponsesRequest(chatCompletionRequest{
		Model: "gpt-5.3-codex",
		Messages: []chatMessage{
			{Role: "tool", Content: `{"ok":true}`},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for tool message without tool_call_id")
	}
	if !strings.Contains(err.Error(), "tool_call_id") {
		t.Fatalf("expected tool_call_id error, got %v", err)
	}
}

func TestCodexCompat_ChatCompletionsNonStreamWithToolCalls(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("parse request body: %v", err)
		}

		if got, ok := req["max_output_tokens"]; ok {
			t.Fatalf("expected max_output_tokens to be omitted, got %v", got)
		}

		reasoning, ok := req["reasoning"].(map[string]any)
		if !ok || reasoning["effort"] != "high" {
			t.Fatalf("expected reasoning.effort=high, got %v", req["reasoning"])
		}

		if got := req["parallel_tool_calls"]; got != true {
			t.Fatalf("expected parallel_tool_calls=true, got %v", got)
		}

		toolChoice, ok := req["tool_choice"].(map[string]any)
		if !ok || toolChoice["type"] != "function" || toolChoice["name"] != "lookup_weather" {
			t.Fatalf("expected mapped function tool_choice, got %v", req["tool_choice"])
		}

		tools, ok := req["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("expected one tool in converted request, got %v", req["tools"])
		}

		input, ok := req["input"].([]any)
		if !ok {
			t.Fatalf("expected input array, got %T", req["input"])
		}

		foundToolOutput := false
		for _, raw := range input {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if item["type"] == "function_call_output" && item["call_id"] == "call_lookup_1" {
				foundToolOutput = true
			}
		}
		if !foundToolOutput {
			t.Fatalf("expected function_call_output in input, got %v", input)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: response.created\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_tool_1\",\"created_at\":1700000002,\"model\":\"gpt-5.3-codex\"}}\n\n"))
		_, _ = w.Write([]byte("event: response.output_item.added\n"))
		_, _ = w.Write([]byte("data: {\"output_index\":0,\"item\":{\"id\":\"fc_2\",\"type\":\"function_call\",\"call_id\":\"call_lookup_2\",\"name\":\"lookup_weather\"}}\n\n"))
		_, _ = w.Write([]byte("event: response.function_call_arguments.delta\n"))
		_, _ = w.Write([]byte("data: {\"item_id\":\"fc_2\",\"output_index\":0,\"delta\":\"{\\\"city\\\":\\\"\"}\n\n"))
		_, _ = w.Write([]byte("event: response.function_call_arguments.delta\n"))
		_, _ = w.Write([]byte("data: {\"item_id\":\"fc_2\",\"output_index\":0,\"delta\":\"SF\\\"}\"}\n\n"))
		_, _ = w.Write([]byte("event: response.completed\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_tool_1\",\"created_at\":1700000002,\"model\":\"gpt-5.3-codex\",\"usage\":{\"input_tokens\":15,\"output_tokens\":4,\"total_tokens\":19}}}\n\n"))
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

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.3-codex","messages":[{"role":"user","content":"Call lookup_weather"},{"role":"assistant","tool_calls":[{"id":"call_lookup_1","type":"function","function":{"name":"lookup_weather","arguments":"{\\\"city\\\":\\\"SF\\\"}"}}]},{"role":"tool","tool_call_id":"call_lookup_1","content":"{\"temp_c\":18}"},{"role":"user","content":"What did tool return?"}],"tools":[{"type":"function","function":{"name":"lookup_weather","description":"lookup weather by city","parameters":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]},"strict":true}}],"tool_choice":{"type":"function","function":{"name":"lookup_weather"}},"parallel_tool_calls":true,"reasoning_effort":"high","max_tokens":256,"stream":false}`))
	req.Header.Set("Authorization", "Bearer fixed-key")
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}

	body := res.Body.String()
	if !strings.Contains(body, `"finish_reason":"tool_calls"`) {
		t.Fatalf("expected finish_reason tool_calls, got: %s", body)
	}
	if !strings.Contains(body, `"tool_calls":[`) {
		t.Fatalf("expected tool_calls in response body, got: %s", body)
	}
	if !strings.Contains(body, `"name":"lookup_weather"`) {
		t.Fatalf("expected tool function name in response body, got: %s", body)
	}
}

func TestCodexCompat_ChatCompletionsStreamWithToolCalls(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: response.created\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_stream_tool\",\"created_at\":1700000003,\"model\":\"gpt-5.3-codex\"}}\n\n"))
		_, _ = w.Write([]byte("event: response.output_item.added\n"))
		_, _ = w.Write([]byte("data: {\"output_index\":0,\"item\":{\"id\":\"fc_stream\",\"type\":\"function_call\",\"call_id\":\"call_stream_1\",\"name\":\"lookup_weather\"}}\n\n"))
		_, _ = w.Write([]byte("event: response.function_call_arguments.delta\n"))
		_, _ = w.Write([]byte("data: {\"item_id\":\"fc_stream\",\"output_index\":0,\"delta\":\"{\\\"city\\\":\\\"SF\\\"}\"}\n\n"))
		_, _ = w.Write([]byte("event: response.completed\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_stream_tool\",\"created_at\":1700000003,\"model\":\"gpt-5.3-codex\"}}\n\n"))
	}))
	defer up.Close()

	h := New(Dependencies{
		FixedAPIKey:        "fixed-key",
		CodexCompat:        true,
		CodexResponsesPath: "/backend-api/codex/responses",
		TokenProvider:      staticTokenProvider{token: fakeTokenWithAccount("acct-123")},
		UpstreamClient:     upstream.NewClient(up.URL, 0),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.3-codex","messages":[{"role":"user","content":"Call lookup_weather"}],"tools":[{"type":"function","function":{"name":"lookup_weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}],"stream":true}`))
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
	if !strings.Contains(body, `"tool_calls":[{`) {
		t.Fatalf("expected streamed tool_calls delta, got: %s", body)
	}
	if !strings.Contains(body, `"finish_reason":"tool_calls"`) {
		t.Fatalf("expected final finish_reason=tool_calls, got: %s", body)
	}
	if !strings.Contains(body, "[DONE]") {
		t.Fatalf("expected [DONE], got: %s", body)
	}
}

func fakeTokenWithAccount(accountID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"` + accountID + `"}}`))
	return header + "." + payload + ".sig"
}

package server

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.deps.CodexCompat {
		writeCodexModels(w)
		return
	}

	s.proxy(w, r, http.MethodGet, s.deps.ModelsPath, nil, "", nil)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "failed to read request body")
		return
	}

	if s.deps.CodexCompat {
		s.handleChatCompletionsCodex(w, r, body)
		return
	}

	s.proxy(w, r, http.MethodPost, s.deps.ChatCompletionsPath, body, r.Header.Get("Content-Type"), nil)
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "failed to read request body")
		return
	}

	if len(strings.TrimSpace(string(body))) > 0 && !json.Valid(body) {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "invalid request body json")
		return
	}

	if s.deps.CodexCompat {
		s.proxy(
			w,
			r,
			http.MethodPost,
			s.deps.CodexResponsesPath,
			body,
			r.Header.Get("Content-Type"),
			map[string]string{"originator": s.deps.CodexOriginator},
		)
		return
	}

	s.proxy(w, r, http.MethodPost, s.deps.ResponsesPath, body, r.Header.Get("Content-Type"), nil)
}

func (s *Server) handleChatCompletionsCodex(w http.ResponseWriter, r *http.Request, rawBody []byte) {
	var chatReq chatCompletionRequest
	if err := json.Unmarshal(rawBody, &chatReq); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "invalid chat completion request json")
		return
	}

	codexReq, err := toCodexResponsesRequest(chatReq)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	body, err := json.Marshal(codexReq)
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "failed to encode upstream request")
		return
	}

	extraHeaders := map[string]string{
		"originator": s.deps.CodexOriginator,
	}

	resp, usedToken, err := s.callUpstream(r.Context(), http.MethodPost, s.deps.CodexResponsesPath, body, "application/json", extraHeaders)
	if err != nil {
		status, code, message := mapGatewayError(err)
		writeOpenAIError(w, status, code, message)
		return
	}
	defer resp.Body.Close()

	if accountID := extractChatGPTAccountID(usedToken); accountID != "" {
		_ = accountID
	}

	if resp.StatusCode >= http.StatusBadRequest {
		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
	}

	if chatReq.Stream {
		s.streamCodexAsChatCompletions(w, resp.Body, chatReq.Model)
		return
	}

	s.writeCodexAsChatCompletionJSON(w, resp.Body, chatReq.Model)
}

func (s *Server) proxy(w http.ResponseWriter, r *http.Request, method, path string, body []byte, contentType string, headers map[string]string) {
	resp, _, err := s.callUpstream(r.Context(), method, path, body, contentType, headers)
	if err != nil {
		status, code, message := mapGatewayError(err)
		writeOpenAIError(w, status, code, message)
		return
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		resp.Body.Close()
		writeOpenAIError(w, http.StatusBadGateway, "upstream_error", "upstream service error")
		return
	}

	relayUpstreamResponse(w, resp)
}

func (s *Server) callUpstream(ctx context.Context, method, path string, body []byte, contentType string, headers map[string]string) (*http.Response, string, error) {
	token, err := s.deps.TokenProvider.AccessToken(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("oauth token unavailable")
	}

	headersWithAccount := cloneHeaders(headers)
	if accountID := extractChatGPTAccountID(token); accountID != "" {
		headersWithAccount["ChatGPT-Account-Id"] = accountID
	}

	resp, err := s.deps.UpstreamClient.Do(ctx, method, path, body, contentType, token, headersWithAccount)
	if err != nil {
		return nil, "", fmt.Errorf("upstream request failed")
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, token, nil
	}

	resp.Body.Close()
	refreshed, refreshErr := s.deps.TokenProvider.ForceRefresh(ctx)
	if refreshErr != nil {
		return nil, "", fmt.Errorf("oauth token unavailable")
	}

	headersWithAccount = cloneHeaders(headers)
	if accountID := extractChatGPTAccountID(refreshed); accountID != "" {
		headersWithAccount["ChatGPT-Account-Id"] = accountID
	}

	resp, err = s.deps.UpstreamClient.Do(ctx, method, path, body, contentType, refreshed, headersWithAccount)
	if err != nil {
		return nil, "", fmt.Errorf("upstream request failed")
	}

	return resp, refreshed, nil
}

func mapGatewayError(err error) (status int, code, message string) {
	if strings.Contains(err.Error(), "oauth token unavailable") {
		return http.StatusServiceUnavailable, "oauth_unavailable", "oauth token unavailable, run auth login"
	}
	return http.StatusBadGateway, "upstream_unavailable", "upstream request failed"
}

func cloneHeaders(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

type chatCompletionRequest struct {
	Model       string           `json:"model"`
	Messages    []chatMessage    `json:"messages"`
	Stream      bool             `json:"stream"`
	Temperature *float64         `json:"temperature,omitempty"`
	TopP        *float64         `json:"top_p,omitempty"`
	MaxTokens   *int             `json:"max_tokens,omitempty"`
	ToolChoice  any              `json:"tool_choice,omitempty"`
	Tools       []map[string]any `json:"tools,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content,omitempty"`
	Name    string `json:"name,omitempty"`
}

type codexResponsesRequest struct {
	Model        string           `json:"model"`
	Instructions string           `json:"instructions"`
	Input        []map[string]any `json:"input"`
	Store        bool             `json:"store"`
	Stream       bool             `json:"stream"`
	Temperature  *float64         `json:"temperature,omitempty"`
	TopP         *float64         `json:"top_p,omitempty"`
}

func toCodexResponsesRequest(in chatCompletionRequest) (codexResponsesRequest, error) {
	model := strings.TrimSpace(in.Model)
	if model == "" {
		return codexResponsesRequest{}, fmt.Errorf("model is required")
	}

	instructions := []string{}
	input := []map[string]any{}
	for _, msg := range in.Messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			continue
		}
		if role == "system" {
			if text := stringifyContent(msg.Content); text != "" {
				instructions = append(instructions, text)
			}
			continue
		}
		item := map[string]any{"role": role}
		if msg.Content != nil {
			item["content"] = msg.Content
		}
		if msg.Name != "" {
			item["name"] = msg.Name
		}
		input = append(input, item)
	}

	if len(input) == 0 {
		return codexResponsesRequest{}, fmt.Errorf("at least one non-system message is required")
	}

	instructionText := strings.TrimSpace(strings.Join(instructions, "\n\n"))
	if instructionText == "" {
		instructionText = "You are a helpful assistant."
	}

	out := codexResponsesRequest{
		Model:        model,
		Instructions: instructionText,
		Input:        input,
		Store:        false,
		Stream:       true,
		Temperature:  in.Temperature,
		TopP:         in.TopP,
	}

	return out, nil
}

func stringifyContent(content any) string {
	switch v := content.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := []string{}
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if text, ok := m["text"].(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func writeCodexModels(w http.ResponseWriter) {
	type model struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	out := struct {
		Object string  `json:"object"`
		Data   []model `json:"data"`
	}{
		Object: "list",
		Data: []model{
			{ID: "gpt-5.3-codex", Object: "model", Created: 0, OwnedBy: "openai"},
			{ID: "gpt-5.2-codex", Object: "model", Created: 0, OwnedBy: "openai"},
			{ID: "gpt-5.1-codex", Object: "model", Created: 0, OwnedBy: "openai"},
			{ID: "gpt-5.1-codex-mini", Object: "model", Created: 0, OwnedBy: "openai"},
			{ID: "gpt-5.1-codex-max", Object: "model", Created: 0, OwnedBy: "openai"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

func extractChatGPTAccountID(accessToken string) string {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}

	authClaims, ok := claims["https://api.openai.com/auth"].(map[string]any)
	if !ok {
		return ""
	}

	id, _ := authClaims["chatgpt_account_id"].(string)
	return strings.TrimSpace(id)
}

type sseEvent struct {
	Event string
	Data  string
}

func parseSSE(reader io.Reader, onEvent func(sseEvent) error) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	currentEvent := ""
	dataLines := []string{}
	emit := func() error {
		if len(dataLines) == 0 {
			currentEvent = ""
			return nil
		}
		err := onEvent(sseEvent{Event: currentEvent, Data: strings.Join(dataLines, "\n")})
		currentEvent = ""
		dataLines = nil
		return err
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := emit(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return emit()
}

func (s *Server) writeCodexAsChatCompletionJSON(w http.ResponseWriter, body io.Reader, requestedModel string) {
	type responseCreated struct {
		Response struct {
			ID        string `json:"id"`
			CreatedAt int64  `json:"created_at"`
			Model     string `json:"model"`
		} `json:"response"`
	}
	type responseDelta struct {
		Delta string `json:"delta"`
	}
	type responseCompleted struct {
		Response struct {
			ID        string `json:"id"`
			CreatedAt int64  `json:"created_at"`
			Model     string `json:"model"`
			Usage     struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
				TotalTokens  int `json:"total_tokens"`
			} `json:"usage"`
		} `json:"response"`
	}

	created := responseCreated{}
	completed := responseCompleted{}
	var textBuilder strings.Builder

	err := parseSSE(body, func(ev sseEvent) error {
		switch ev.Event {
		case "response.created":
			_ = json.Unmarshal([]byte(ev.Data), &created)
		case "response.output_text.delta":
			var delta responseDelta
			if err := json.Unmarshal([]byte(ev.Data), &delta); err == nil {
				textBuilder.WriteString(delta.Delta)
			}
		case "response.completed":
			_ = json.Unmarshal([]byte(ev.Data), &completed)
		}
		return nil
	})
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, "upstream_error", "failed to parse upstream stream response")
		return
	}

	model := completed.Response.Model
	if model == "" {
		model = created.Response.Model
	}
	if model == "" {
		model = requestedModel
	}

	id := completed.Response.ID
	if id == "" {
		id = created.Response.ID
	}
	if id == "" {
		id = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}

	createdAt := completed.Response.CreatedAt
	if createdAt == 0 {
		createdAt = created.Response.CreatedAt
	}
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}

	response := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": createdAt,
		"model":   model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": textBuilder.String(),
				},
				"finish_reason": "stop",
			},
		},
	}

	if completed.Response.Usage.TotalTokens > 0 {
		response["usage"] = map[string]any{
			"prompt_tokens":     completed.Response.Usage.InputTokens,
			"completion_tokens": completed.Response.Usage.OutputTokens,
			"total_tokens":      completed.Response.Usage.TotalTokens,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) streamCodexAsChatCompletions(w http.ResponseWriter, body io.Reader, requestedModel string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeOpenAIError(w, http.StatusInternalServerError, "stream_not_supported", "streaming is not supported by this server")
		return
	}

	type responseCreated struct {
		Response struct {
			ID        string `json:"id"`
			CreatedAt int64  `json:"created_at"`
			Model     string `json:"model"`
		} `json:"response"`
	}
	type responseDelta struct {
		Delta string `json:"delta"`
	}

	id := ""
	createdAt := int64(0)
	model := requestedModel
	roleSent := false

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	err := parseSSE(body, func(ev sseEvent) error {
		switch ev.Event {
		case "response.created":
			var created responseCreated
			if err := json.Unmarshal([]byte(ev.Data), &created); err == nil {
				if created.Response.ID != "" {
					id = created.Response.ID
				}
				if created.Response.CreatedAt > 0 {
					createdAt = created.Response.CreatedAt
				}
				if created.Response.Model != "" {
					model = created.Response.Model
				}
			}
		case "response.output_text.delta":
			var delta responseDelta
			if err := json.Unmarshal([]byte(ev.Data), &delta); err != nil {
				return nil
			}
			if delta.Delta == "" {
				return nil
			}
			if id == "" {
				id = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
			}
			if createdAt == 0 {
				createdAt = time.Now().Unix()
			}
			deltaObj := map[string]any{"content": delta.Delta}
			if !roleSent {
				deltaObj["role"] = "assistant"
				roleSent = true
			}
			chunk := map[string]any{
				"id":      id,
				"object":  "chat.completion.chunk",
				"created": createdAt,
				"model":   model,
				"choices": []map[string]any{{
					"index":         0,
					"delta":         deltaObj,
					"finish_reason": nil,
				}},
			}
			b, _ := json.Marshal(chunk)
			_, _ = w.Write([]byte("data: " + string(b) + "\n\n"))
			flusher.Flush()
		case "response.completed":
			if id == "" {
				id = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
			}
			if createdAt == 0 {
				createdAt = time.Now().Unix()
			}
			final := map[string]any{
				"id":      id,
				"object":  "chat.completion.chunk",
				"created": createdAt,
				"model":   model,
				"choices": []map[string]any{{
					"index":         0,
					"delta":         map[string]any{},
					"finish_reason": "stop",
				}},
			}
			b, _ := json.Marshal(final)
			_, _ = w.Write([]byte("data: " + string(b) + "\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
		}
		return nil
	})

	if err != nil {
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}
}

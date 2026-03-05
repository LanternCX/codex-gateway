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
		codexBody, err := normalizeCodexResponsesRequest(body)
		if err != nil {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "invalid request body json")
			return
		}

		s.proxy(
			w,
			r,
			http.MethodPost,
			s.deps.CodexResponsesPath,
			codexBody,
			r.Header.Get("Content-Type"),
			map[string]string{"originator": s.deps.CodexOriginator},
		)
		return
	}

	s.proxy(w, r, http.MethodPost, s.deps.ResponsesPath, body, r.Header.Get("Content-Type"), nil)
}

func normalizeCodexResponsesRequest(body []byte) ([]byte, error) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return body, nil
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode responses request: %w", err)
	}

	obj, ok := payload.(map[string]any)
	if !ok {
		return body, nil
	}

	mutated := false

	if _, ok := obj["max_output_tokens"]; ok {
		delete(obj, "max_output_tokens")
		mutated = true
	}

	if _, ok := obj["max_completion_tokens"]; ok {
		delete(obj, "max_completion_tokens")
		mutated = true
	}

	if hasCodexInstructions(obj["instructions"]) && !mutated {
		return body, nil
	}

	if !hasCodexInstructions(obj["instructions"]) {
		obj["instructions"] = "You are a helpful assistant."
		mutated = true
	}

	if !mutated {
		return body, nil
	}

	normalizedBody, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("encode responses request: %w", err)
	}

	return normalizedBody, nil
}

func hasCodexInstructions(value any) bool {
	v, ok := value.(string)
	if !ok {
		return false
	}

	return strings.TrimSpace(v) != ""
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
	Model               string           `json:"model"`
	Messages            []chatMessage    `json:"messages"`
	Stream              bool             `json:"stream"`
	Temperature         *float64         `json:"temperature,omitempty"`
	TopP                *float64         `json:"top_p,omitempty"`
	MaxTokens           *int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int             `json:"max_completion_tokens,omitempty"`
	ToolChoice          any              `json:"tool_choice,omitempty"`
	Tools               []map[string]any `json:"tools,omitempty"`
	ParallelToolCalls   *bool            `json:"parallel_tool_calls,omitempty"`
	ReasoningEffort     string           `json:"reasoning_effort,omitempty"`
	Functions           []map[string]any `json:"functions,omitempty"`
	FunctionCall        any              `json:"function_call,omitempty"`
}

type chatMessage struct {
	Role         string            `json:"role"`
	Content      any               `json:"content,omitempty"`
	Name         string            `json:"name,omitempty"`
	ToolCallID   string            `json:"tool_call_id,omitempty"`
	ToolCalls    []chatToolCall    `json:"tool_calls,omitempty"`
	FunctionCall *chatFunctionCall `json:"function_call,omitempty"`
}

type chatToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function chatFunctionCall `json:"function"`
}

type chatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type codexResponsesRequest struct {
	Model             string           `json:"model"`
	Instructions      string           `json:"instructions"`
	Input             []map[string]any `json:"input"`
	Store             bool             `json:"store"`
	Stream            bool             `json:"stream"`
	Temperature       *float64         `json:"temperature,omitempty"`
	TopP              *float64         `json:"top_p,omitempty"`
	Tools             []map[string]any `json:"tools,omitempty"`
	ToolChoice        any              `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool            `json:"parallel_tool_calls,omitempty"`
	Reasoning         *codexReasoning  `json:"reasoning,omitempty"`
}

type codexReasoning struct {
	Effort string `json:"effort,omitempty"`
}

func toCodexResponsesRequest(in chatCompletionRequest) (codexResponsesRequest, error) {
	model := strings.TrimSpace(in.Model)
	if model == "" {
		return codexResponsesRequest{}, fmt.Errorf("model is required")
	}

	instructions := []string{}
	input := []map[string]any{}
	legacyCallCount := 0
	for i, msg := range in.Messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			continue
		}

		switch role {
		case "system":
			if text := stringifyContent(msg.Content); text != "" {
				instructions = append(instructions, text)
			}
		case "tool":
			toolCallID := strings.TrimSpace(msg.ToolCallID)
			if toolCallID == "" {
				return codexResponsesRequest{}, fmt.Errorf("tool message requires tool_call_id")
			}
			input = append(input, map[string]any{
				"type":    "function_call_output",
				"call_id": toolCallID,
				"output":  toCodexFunctionCallOutput(msg.Content),
			})
		case "assistant":
			assistantCallItems, err := toCodexAssistantToolCallItems(msg, i, &legacyCallCount)
			if err != nil {
				return codexResponsesRequest{}, err
			}
			input = append(input, assistantCallItems...)

			if msg.Content != nil || strings.TrimSpace(msg.Name) != "" {
				item := map[string]any{"role": "assistant"}
				if msg.Content != nil {
					item["content"] = msg.Content
				}
				if msg.Name != "" {
					item["name"] = msg.Name
				}
				input = append(input, item)
			}
		default:
			item := map[string]any{"role": role}
			if msg.Content != nil {
				item["content"] = msg.Content
			}
			if msg.Name != "" {
				item["name"] = msg.Name
			}
			input = append(input, item)
		}
	}

	if len(input) == 0 {
		return codexResponsesRequest{}, fmt.Errorf("at least one non-system message is required")
	}

	instructionText := strings.TrimSpace(strings.Join(instructions, "\n\n"))
	if instructionText == "" {
		instructionText = "You are a helpful assistant."
	}

	tools, err := toCodexTools(in.Tools, in.Functions)
	if err != nil {
		return codexResponsesRequest{}, err
	}

	toolChoice, err := toCodexToolChoice(in.ToolChoice, in.FunctionCall)
	if err != nil {
		return codexResponsesRequest{}, err
	}

	out := codexResponsesRequest{
		Model:             model,
		Instructions:      instructionText,
		Input:             input,
		Store:             false,
		Stream:            true,
		Temperature:       in.Temperature,
		TopP:              in.TopP,
		Tools:             tools,
		ToolChoice:        toolChoice,
		ParallelToolCalls: in.ParallelToolCalls,
		Reasoning:         toCodexReasoning(in.ReasoningEffort),
	}

	return out, nil
}

func toCodexAssistantToolCallItems(msg chatMessage, messageIndex int, legacyCallCount *int) ([]map[string]any, error) {
	items := []map[string]any{}

	for i, call := range msg.ToolCalls {
		callType := strings.TrimSpace(call.Type)
		if callType == "" {
			callType = "function"
		}
		if callType != "function" {
			continue
		}

		name := strings.TrimSpace(call.Function.Name)
		if name == "" {
			return nil, fmt.Errorf("assistant tool_calls[%d].function.name is required", i)
		}

		callID := strings.TrimSpace(call.ID)
		if callID == "" {
			callID = fmt.Sprintf("call_m%d_t%d", messageIndex, i)
		}

		arguments := call.Function.Arguments
		if strings.TrimSpace(arguments) == "" {
			arguments = "{}"
		}

		items = append(items, map[string]any{
			"type":      "function_call",
			"call_id":   callID,
			"name":      name,
			"arguments": arguments,
		})
	}

	if msg.FunctionCall == nil {
		return items, nil
	}

	name := strings.TrimSpace(msg.FunctionCall.Name)
	if name == "" {
		return nil, fmt.Errorf("assistant function_call.name is required")
	}

	arguments := msg.FunctionCall.Arguments
	if strings.TrimSpace(arguments) == "" {
		arguments = "{}"
	}

	*legacyCallCount = *legacyCallCount + 1
	items = append(items, map[string]any{
		"type":      "function_call",
		"call_id":   fmt.Sprintf("call_legacy_%d", *legacyCallCount),
		"name":      name,
		"arguments": arguments,
	})

	return items, nil
}

func toCodexFunctionCallOutput(content any) any {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		return v
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(encoded)
	}
}

func toCodexTools(chatTools []map[string]any, legacyFunctions []map[string]any) ([]map[string]any, error) {
	if len(chatTools) == 0 && len(legacyFunctions) > 0 {
		chatTools = make([]map[string]any, 0, len(legacyFunctions))
		for i, function := range legacyFunctions {
			name := strings.TrimSpace(asString(function["name"]))
			if name == "" {
				return nil, fmt.Errorf("functions[%d].name is required", i)
			}

			toolFunction := map[string]any{
				"name": name,
			}
			if desc, ok := function["description"]; ok {
				toolFunction["description"] = desc
			}
			if params, ok := function["parameters"]; ok {
				toolFunction["parameters"] = params
			}

			chatTools = append(chatTools, map[string]any{
				"type":     "function",
				"function": toolFunction,
			})
		}
	}

	if len(chatTools) == 0 {
		return nil, nil
	}

	tools := make([]map[string]any, 0, len(chatTools))
	for i, tool := range chatTools {
		toolType := strings.TrimSpace(asString(tool["type"]))
		if toolType == "" {
			return nil, fmt.Errorf("tools[%d].type is required", i)
		}

		if toolType != "function" {
			tools = append(tools, tool)
			continue
		}

		rawFunction, ok := tool["function"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("tools[%d].function is required for function tools", i)
		}

		name := strings.TrimSpace(asString(rawFunction["name"]))
		if name == "" {
			return nil, fmt.Errorf("tools[%d].function.name is required", i)
		}

		mappedTool := map[string]any{
			"type": "function",
			"name": name,
		}
		if description, ok := rawFunction["description"]; ok {
			mappedTool["description"] = description
		}
		if parameters, ok := rawFunction["parameters"]; ok {
			mappedTool["parameters"] = parameters
		} else {
			mappedTool["parameters"] = nil
		}
		if strict, ok := rawFunction["strict"]; ok {
			mappedTool["strict"] = strict
		} else {
			mappedTool["strict"] = false
		}

		tools = append(tools, mappedTool)
	}

	return tools, nil
}

func toCodexToolChoice(toolChoice any, legacyFunctionCall any) (any, error) {
	choice := toolChoice
	if choice == nil {
		choice = legacyFunctionCall
	}
	if choice == nil {
		return nil, nil
	}

	switch v := choice.(type) {
	case string:
		value := strings.TrimSpace(v)
		if value == "" {
			return nil, nil
		}
		return value, nil
	case map[string]any:
		if function, ok := v["function"].(map[string]any); ok {
			name := strings.TrimSpace(asString(function["name"]))
			if name == "" {
				return nil, fmt.Errorf("tool_choice.function.name is required")
			}
			return map[string]any{"type": "function", "name": name}, nil
		}

		if strings.TrimSpace(asString(v["type"])) == "function" {
			name := strings.TrimSpace(asString(v["name"]))
			if name == "" {
				return nil, fmt.Errorf("tool_choice.name is required for type=function")
			}
			return map[string]any{"type": "function", "name": name}, nil
		}

		if name := strings.TrimSpace(asString(v["name"])); name != "" {
			return map[string]any{"type": "function", "name": name}, nil
		}

		return v, nil
	default:
		return choice, nil
	}
}

func toCodexReasoning(effort string) *codexReasoning {
	value := strings.TrimSpace(effort)
	if value == "" {
		return nil
	}
	return &codexReasoning{Effort: value}
}

func asString(value any) string {
	s, _ := value.(string)
	return s
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

type codexResponseCreatedEvent struct {
	Response struct {
		ID        string `json:"id"`
		CreatedAt int64  `json:"created_at"`
		Model     string `json:"model"`
	} `json:"response"`
}

type codexResponseOutputTextDeltaEvent struct {
	Delta string `json:"delta"`
}

type codexResponseOutputItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type codexResponseOutputItemEvent struct {
	OutputIndex int                     `json:"output_index"`
	Item        codexResponseOutputItem `json:"item"`
}

type codexResponseFunctionCallArgumentsDeltaEvent struct {
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	Delta       string `json:"delta"`
}

type codexResponseFunctionCallArgumentsDoneEvent struct {
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	Name        string `json:"name"`
	Arguments   string `json:"arguments"`
}

type codexResponseCompletedEvent struct {
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

type codexToolCallState struct {
	OutputIndex      int
	ToolIndex        int
	ItemID           string
	CallID           string
	Name             string
	Arguments        string
	MetadataSent     bool
	BufferedArgs     strings.Builder
	ArgumentsEmitted bool
}

func getOrCreateCodexToolCallState(byOutputIndex map[int]*codexToolCallState, byItemID map[string]*codexToolCallState, ordered *[]*codexToolCallState, outputIndex int, itemID string) *codexToolCallState {
	trimmedItemID := strings.TrimSpace(itemID)
	if trimmedItemID != "" {
		if state, ok := byItemID[trimmedItemID]; ok {
			if _, exists := byOutputIndex[outputIndex]; !exists {
				byOutputIndex[outputIndex] = state
			}
			return state
		}
	}

	if state, ok := byOutputIndex[outputIndex]; ok {
		if trimmedItemID != "" {
			state.ItemID = trimmedItemID
			byItemID[trimmedItemID] = state
		}
		return state
	}

	state := &codexToolCallState{
		OutputIndex: outputIndex,
		ToolIndex:   len(*ordered),
		ItemID:      trimmedItemID,
	}
	byOutputIndex[outputIndex] = state
	if trimmedItemID != "" {
		byItemID[trimmedItemID] = state
	}
	*ordered = append(*ordered, state)
	return state
}

func applyCodexOutputItemToToolCallState(state *codexToolCallState, item codexResponseOutputItem) {
	if id := strings.TrimSpace(item.ID); id != "" {
		state.ItemID = id
	}
	if callID := strings.TrimSpace(item.CallID); callID != "" {
		state.CallID = callID
	}
	if name := strings.TrimSpace(item.Name); name != "" {
		state.Name = name
	}
	if strings.TrimSpace(state.Arguments) == "" && item.Arguments != "" {
		state.Arguments = item.Arguments
	}
}

func buildChatToolCalls(states []*codexToolCallState) []map[string]any {
	if len(states) == 0 {
		return nil
	}

	out := make([]map[string]any, 0, len(states))
	for i, state := range states {
		toolID := strings.TrimSpace(state.CallID)
		if toolID == "" {
			toolID = strings.TrimSpace(state.ItemID)
		}
		if toolID == "" {
			toolID = fmt.Sprintf("call_%d", i+1)
		}

		name := strings.TrimSpace(state.Name)
		if name == "" {
			name = "unknown_function"
		}

		arguments := state.Arguments
		if strings.TrimSpace(arguments) == "" {
			arguments = "{}"
		}

		out = append(out, map[string]any{
			"id":   toolID,
			"type": "function",
			"function": map[string]any{
				"name":      name,
				"arguments": arguments,
			},
		})
	}

	return out
}

func (s *Server) writeCodexAsChatCompletionJSON(w http.ResponseWriter, body io.Reader, requestedModel string) {
	created := codexResponseCreatedEvent{}
	completed := codexResponseCompletedEvent{}
	var textBuilder strings.Builder
	toolCallsByOutputIndex := map[int]*codexToolCallState{}
	toolCallsByItemID := map[string]*codexToolCallState{}
	orderedToolCalls := []*codexToolCallState{}

	err := parseSSE(body, func(ev sseEvent) error {
		switch ev.Event {
		case "response.created":
			_ = json.Unmarshal([]byte(ev.Data), &created)
		case "response.output_text.delta":
			var delta codexResponseOutputTextDeltaEvent
			if err := json.Unmarshal([]byte(ev.Data), &delta); err == nil {
				textBuilder.WriteString(delta.Delta)
			}
		case "response.output_item.added", "response.output_item.done":
			var itemEvent codexResponseOutputItemEvent
			if err := json.Unmarshal([]byte(ev.Data), &itemEvent); err != nil {
				return nil
			}
			if itemEvent.Item.Type != "function_call" {
				return nil
			}

			state := getOrCreateCodexToolCallState(
				toolCallsByOutputIndex,
				toolCallsByItemID,
				&orderedToolCalls,
				itemEvent.OutputIndex,
				itemEvent.Item.ID,
			)
			applyCodexOutputItemToToolCallState(state, itemEvent.Item)
		case "response.function_call_arguments.delta":
			var argsDelta codexResponseFunctionCallArgumentsDeltaEvent
			if err := json.Unmarshal([]byte(ev.Data), &argsDelta); err != nil {
				return nil
			}

			state := getOrCreateCodexToolCallState(
				toolCallsByOutputIndex,
				toolCallsByItemID,
				&orderedToolCalls,
				argsDelta.OutputIndex,
				argsDelta.ItemID,
			)
			state.Arguments += argsDelta.Delta
		case "response.function_call_arguments.done":
			var argsDone codexResponseFunctionCallArgumentsDoneEvent
			if err := json.Unmarshal([]byte(ev.Data), &argsDone); err != nil {
				return nil
			}

			state := getOrCreateCodexToolCallState(
				toolCallsByOutputIndex,
				toolCallsByItemID,
				&orderedToolCalls,
				argsDone.OutputIndex,
				argsDone.ItemID,
			)
			if name := strings.TrimSpace(argsDone.Name); name != "" {
				state.Name = name
			}
			if argsDone.Arguments != "" {
				state.Arguments = argsDone.Arguments
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

	chatToolCalls := buildChatToolCalls(orderedToolCalls)

	messageContent := any(textBuilder.String())
	if textBuilder.Len() == 0 && len(chatToolCalls) > 0 {
		messageContent = nil
	}

	message := map[string]any{
		"role":    "assistant",
		"content": messageContent,
	}
	finishReason := "stop"
	if len(chatToolCalls) > 0 {
		message["tool_calls"] = chatToolCalls
		finishReason = "tool_calls"
	}

	response := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": createdAt,
		"model":   model,
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       message,
				"finish_reason": finishReason,
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

	id := ""
	createdAt := int64(0)
	model := requestedModel
	roleSent := false
	toolCallsSeen := false

	toolCallsByOutputIndex := map[int]*codexToolCallState{}
	toolCallsByItemID := map[string]*codexToolCallState{}
	orderedToolCalls := []*codexToolCallState{}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	emitChunk := func(delta map[string]any, finishReason any) {
		if id == "" {
			id = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
		}
		if createdAt == 0 {
			createdAt = time.Now().Unix()
		}
		if strings.TrimSpace(model) == "" {
			model = requestedModel
		}

		if delta != nil && !roleSent {
			if _, ok := delta["role"]; !ok {
				delta["role"] = "assistant"
			}
			roleSent = true
		}

		chunk := map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": createdAt,
			"model":   model,
			"choices": []map[string]any{{
				"index":         0,
				"delta":         delta,
				"finish_reason": finishReason,
			}},
		}
		b, _ := json.Marshal(chunk)
		_, _ = w.Write([]byte("data: " + string(b) + "\n\n"))
		flusher.Flush()
	}

	emitToolCallArguments := func(state *codexToolCallState, argumentDelta string) {
		if argumentDelta == "" {
			return
		}

		emitChunk(map[string]any{
			"tool_calls": []map[string]any{{
				"index": state.ToolIndex,
				"function": map[string]any{
					"arguments": argumentDelta,
				},
			}},
		}, nil)
		state.ArgumentsEmitted = true
	}

	emitToolCallMetadata := func(state *codexToolCallState, force bool) {
		if state.MetadataSent {
			return
		}
		if !force && strings.TrimSpace(state.CallID) == "" && strings.TrimSpace(state.Name) == "" {
			return
		}

		toolCallDelta := map[string]any{
			"index":    state.ToolIndex,
			"type":     "function",
			"function": map[string]any{},
		}
		if callID := strings.TrimSpace(state.CallID); callID != "" {
			toolCallDelta["id"] = callID
		}
		if name := strings.TrimSpace(state.Name); name != "" {
			toolCallDelta["function"] = map[string]any{"name": name}
		}

		emitChunk(map[string]any{"tool_calls": []map[string]any{toolCallDelta}}, nil)
		state.MetadataSent = true

		if state.BufferedArgs.Len() > 0 {
			emitToolCallArguments(state, state.BufferedArgs.String())
			state.BufferedArgs.Reset()
		}
	}

	err := parseSSE(body, func(ev sseEvent) error {
		switch ev.Event {
		case "response.created":
			var created codexResponseCreatedEvent
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
			var delta codexResponseOutputTextDeltaEvent
			if err := json.Unmarshal([]byte(ev.Data), &delta); err != nil {
				return nil
			}
			if delta.Delta == "" {
				return nil
			}
			emitChunk(map[string]any{"content": delta.Delta}, nil)
		case "response.output_item.added", "response.output_item.done":
			var itemEvent codexResponseOutputItemEvent
			if err := json.Unmarshal([]byte(ev.Data), &itemEvent); err != nil {
				return nil
			}
			if itemEvent.Item.Type != "function_call" {
				return nil
			}

			toolCallsSeen = true
			state := getOrCreateCodexToolCallState(
				toolCallsByOutputIndex,
				toolCallsByItemID,
				&orderedToolCalls,
				itemEvent.OutputIndex,
				itemEvent.Item.ID,
			)
			applyCodexOutputItemToToolCallState(state, itemEvent.Item)
			emitToolCallMetadata(state, true)

			if strings.TrimSpace(itemEvent.Item.Arguments) != "" && !state.ArgumentsEmitted && state.BufferedArgs.Len() == 0 {
				emitToolCallArguments(state, itemEvent.Item.Arguments)
				state.Arguments = itemEvent.Item.Arguments
			}
		case "response.function_call_arguments.delta":
			var argsDelta codexResponseFunctionCallArgumentsDeltaEvent
			if err := json.Unmarshal([]byte(ev.Data), &argsDelta); err != nil {
				return nil
			}
			if argsDelta.Delta == "" {
				return nil
			}

			toolCallsSeen = true
			state := getOrCreateCodexToolCallState(
				toolCallsByOutputIndex,
				toolCallsByItemID,
				&orderedToolCalls,
				argsDelta.OutputIndex,
				argsDelta.ItemID,
			)
			state.Arguments += argsDelta.Delta
			if state.MetadataSent {
				emitToolCallArguments(state, argsDelta.Delta)
			} else {
				state.BufferedArgs.WriteString(argsDelta.Delta)
			}
		case "response.function_call_arguments.done":
			var argsDone codexResponseFunctionCallArgumentsDoneEvent
			if err := json.Unmarshal([]byte(ev.Data), &argsDone); err != nil {
				return nil
			}

			toolCallsSeen = true
			state := getOrCreateCodexToolCallState(
				toolCallsByOutputIndex,
				toolCallsByItemID,
				&orderedToolCalls,
				argsDone.OutputIndex,
				argsDone.ItemID,
			)
			if name := strings.TrimSpace(argsDone.Name); name != "" {
				state.Name = name
			}
			if argsDone.Arguments != "" {
				state.Arguments = argsDone.Arguments
			}

			emitToolCallMetadata(state, true)
			if !state.ArgumentsEmitted && argsDone.Arguments != "" {
				emitToolCallArguments(state, argsDone.Arguments)
			}
		case "response.completed":
			var completed codexResponseCompletedEvent
			if err := json.Unmarshal([]byte(ev.Data), &completed); err == nil {
				if completed.Response.ID != "" {
					id = completed.Response.ID
				}
				if completed.Response.CreatedAt > 0 {
					createdAt = completed.Response.CreatedAt
				}
				if completed.Response.Model != "" {
					model = completed.Response.Model
				}
			}

			finishReason := "stop"
			if toolCallsSeen {
				finishReason = "tool_calls"
			}
			emitChunk(map[string]any{}, finishReason)
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

package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type TokenProvider interface {
	AccessToken(context.Context) (string, error)
	ForceRefresh(context.Context) (string, error)
}

type UpstreamClient interface {
	Do(ctx context.Context, method, path string, body []byte, contentType, accessToken string, headers map[string]string) (*http.Response, error)
}

type Dependencies struct {
	FixedAPIKey         string
	ModelsPath          string
	ChatCompletionsPath string
	CodexCompat         bool
	CodexResponsesPath  string
	CodexOriginator     string
	Logger              *slog.Logger
	TokenProvider       TokenProvider
	UpstreamClient      UpstreamClient
}

type Server struct {
	deps   Dependencies
	logger *slog.Logger
}

func New(deps Dependencies) http.Handler {
	if deps.ModelsPath == "" {
		deps.ModelsPath = "/v1/models"
	}
	if deps.ChatCompletionsPath == "" {
		deps.ChatCompletionsPath = "/v1/chat/completions"
	}
	if deps.CodexResponsesPath == "" {
		deps.CodexResponsesPath = "/backend-api/codex/responses"
	}
	if deps.CodexOriginator == "" {
		deps.CodexOriginator = "opencode"
	}
	if deps.Logger == nil {
		deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	mux := http.NewServeMux()
	srv := &Server{deps: deps, logger: deps.Logger}

	mux.HandleFunc("/healthz", srv.handleHealthz)

	auth := AuthMiddleware(deps.FixedAPIKey)
	mux.Handle(normalizePath(deps.ModelsPath), auth(http.HandlerFunc(srv.handleModels)))
	mux.Handle(normalizePath(deps.ChatCompletionsPath), auth(http.HandlerFunc(srv.handleChatCompletions)))

	return RequestLoggingMiddleware(deps.Logger)(mux)
}

func normalizePath(p string) string {
	if strings.TrimSpace(p) == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

func relayUpstreamResponse(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

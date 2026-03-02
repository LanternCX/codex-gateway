package upstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	gatewaylog "codex-gateway/internal/logging"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

type Option func(*Client)

func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) {
		if logger != nil {
			c.logger = logger
		}
	}
}

func NewClient(baseURL string, timeout time.Duration, opts ...Option) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	client := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func (c *Client) Do(ctx context.Context, method, path string, body []byte, contentType, accessToken string, headers map[string]string) (*http.Response, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	if requestID := gatewaylog.RequestIDFromContext(ctx); requestID != "" {
		req.Header.Set(gatewaylog.RequestIDHeader, requestID)
	}
	for k, v := range headers {
		if k == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.ErrorContext(
			ctx,
			"upstream request failed",
			"method", method,
			"path", path,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err,
		)
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}

	c.logger.DebugContext(
		ctx,
		"upstream response received",
		"method", method,
		"path", path,
		"status", resp.StatusCode,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return resp, nil
}

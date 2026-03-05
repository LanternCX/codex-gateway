package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultListenAddr      = ":8080"
	defaultStartupProbeTTL = 3 * time.Second
)

type modelDiscoverer func(ctx context.Context, apiPrefix, apiKey string) ([]string, error)

func buildAPIPrefix(listen string) string {
	apiPrefix, _, _ := buildServePrefixes(listen)
	return apiPrefix
}

func buildProbeAPIPrefix(listen string) string {
	_, probePrefix, _ := buildServePrefixes(listen)
	return probePrefix
}

func buildServePrefixes(listen string) (apiPrefix string, probePrefix string, parseErr error) {
	host, port, parseErr := splitListenAddress(listen)
	if parseErr != nil {
		return "http://127.0.0.1:8080/v1", "http://localhost:8080/v1", parseErr
	}

	apiHost := normalizePublicListenHost(host)
	probeHost := normalizeProbeListenHost(host)

	return "http://" + net.JoinHostPort(apiHost, port) + "/v1", "http://" + net.JoinHostPort(probeHost, port) + "/v1", nil
}

func splitListenAddress(listen string) (host, port string, err error) {
	trimmed := strings.TrimSpace(listen)
	if trimmed == "" {
		trimmed = defaultListenAddr
	}

	if strings.HasPrefix(trimmed, ":") {
		return "", strings.TrimPrefix(trimmed, ":"), nil
	}

	host, port, err = net.SplitHostPort(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("parse listen address %q: %w", listen, err)
	}

	return host, port, nil
}

func normalizePublicListenHost(host string) string {
	value := strings.TrimSpace(host)
	switch value {
	case "", "0.0.0.0", "::", "[::]":
		return "127.0.0.1"
	default:
		return value
	}
}

func normalizeProbeListenHost(host string) string {
	value := strings.TrimSpace(host)
	switch value {
	case "", "0.0.0.0", "::", "[::]":
		return "localhost"
	default:
		return value
	}
}

func discoverAvailableModels(ctx context.Context, client *http.Client, apiPrefix, apiKey string) ([]string, error) {
	if client == nil {
		client = &http.Client{Timeout: defaultStartupProbeTTL}
	}

	endpoint := strings.TrimSuffix(strings.TrimSpace(apiPrefix), "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build models probe request: %w", err)
	}

	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send models probe request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("models probe returned status %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode models probe response: %w", err)
	}

	seen := map[string]struct{}{}
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}

	return models, nil
}

func logServeStartupInfo(ctx context.Context, logger *slog.Logger, apiPrefix, apiKey string, discover modelDiscoverer, attempts int, retryDelay time.Duration) {
	if logger == nil || discover == nil {
		return
	}

	if attempts <= 0 {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		models, err := discover(ctx, apiPrefix, apiKey)
		if err == nil {
			logger.InfoContext(ctx, "gateway startup models discovered", "api_prefix", apiPrefix, "available_models", models)
			return
		}

		if attempt == attempts {
			logger.WarnContext(ctx, "gateway startup model discovery failed", "api_prefix", apiPrefix, "error", err)
			return
		}

		if retryDelay <= 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(retryDelay):
		}
	}
}

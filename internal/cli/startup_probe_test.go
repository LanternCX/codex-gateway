package cli

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestBuildAPIPrefix(t *testing.T) {
	tests := []struct {
		name   string
		listen string
		want   string
	}{
		{name: "port only", listen: ":8080", want: "http://127.0.0.1:8080/v1"},
		{name: "wildcard host", listen: "0.0.0.0:9000", want: "http://127.0.0.1:9000/v1"},
		{name: "ipv6 wildcard", listen: "[::]:7000", want: "http://127.0.0.1:7000/v1"},
		{name: "explicit localhost", listen: "localhost:7777", want: "http://localhost:7777/v1"},
		{name: "empty uses default", listen: "", want: "http://127.0.0.1:8080/v1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildAPIPrefix(tc.listen)
			if got != tc.want {
				t.Fatalf("unexpected api prefix: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestBuildProbeAPIPrefix(t *testing.T) {
	tests := []struct {
		name   string
		listen string
		want   string
	}{
		{name: "port only", listen: ":8080", want: "http://localhost:8080/v1"},
		{name: "wildcard host", listen: "0.0.0.0:9000", want: "http://localhost:9000/v1"},
		{name: "ipv6 wildcard", listen: "[::]:7000", want: "http://localhost:7000/v1"},
		{name: "explicit localhost", listen: "localhost:7777", want: "http://localhost:7777/v1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildProbeAPIPrefix(tc.listen)
			if got != tc.want {
				t.Fatalf("unexpected probe api prefix: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestBuildServePrefixes_InvalidListenFallsBackAndReturnsError(t *testing.T) {
	apiPrefix, probePrefix, err := buildServePrefixes("invalid-listen")
	if err == nil {
		t.Fatal("expected parse error for invalid listen address")
	}

	if apiPrefix != "http://127.0.0.1:8080/v1" {
		t.Fatalf("unexpected fallback api prefix: %q", apiPrefix)
	}

	if probePrefix != "http://localhost:8080/v1" {
		t.Fatalf("unexpected fallback probe prefix: %q", probePrefix)
	}
}

func TestDiscoverAvailableModels_Success(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if got := r.Header.Get("Authorization"); got != "Bearer fixed-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-5.3-codex"},{"id":"gpt-5.3-codex"},{"id":"gpt-5.2-codex"},{"id":""}]}`))
	}))
	defer up.Close()

	models, err := discoverAvailableModels(context.Background(), up.Client(), up.URL+"/v1", "fixed-key")
	if err != nil {
		t.Fatalf("discover models: %v", err)
	}

	want := []string{"gpt-5.3-codex", "gpt-5.2-codex"}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("unexpected models: got %v want %v", models, want)
	}
}

func TestDiscoverAvailableModels_HTTPError(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer up.Close()

	_, err := discoverAvailableModels(context.Background(), up.Client(), up.URL+"/v1", "fixed-key")
	if err == nil {
		t.Fatal("expected error for upstream non-2xx response")
	}

	if !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("expected status detail in error, got: %v", err)
	}
}

func TestDiscoverAvailableModels_InvalidJSON(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":`))
	}))
	defer up.Close()

	_, err := discoverAvailableModels(context.Background(), up.Client(), up.URL+"/v1", "fixed-key")
	if err == nil {
		t.Fatal("expected error for invalid model response json")
	}
}

func TestLogServeStartupInfo_LogsPrefixAndModels(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	discover := func(ctx context.Context, apiPrefix, apiKey string) ([]string, error) {
		if apiPrefix != "http://127.0.0.1:8080/v1" {
			t.Fatalf("unexpected api prefix: %s", apiPrefix)
		}
		if apiKey != "fixed-key" {
			t.Fatalf("unexpected api key: %s", apiKey)
		}
		return []string{"gpt-5.3-codex", "gpt-5.2-codex"}, nil
	}

	logServeStartupInfo(context.Background(), logger, "http://127.0.0.1:8080/v1", "fixed-key", discover, 1, 0)

	out := buf.String()
	if !strings.Contains(out, `"msg":"gateway startup models discovered"`) {
		t.Fatalf("expected startup model discovery log, got: %s", out)
	}

	if !strings.Contains(out, `"api_prefix":"http://127.0.0.1:8080/v1"`) {
		t.Fatalf("expected api_prefix in log, got: %s", out)
	}

	if !strings.Contains(out, `"available_models":["gpt-5.3-codex","gpt-5.2-codex"]`) {
		t.Fatalf("expected available_models in log, got: %s", out)
	}
}

func TestLogServeStartupInfo_RetriesAndWarnsOnFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	attempts := 0
	discover := func(ctx context.Context, apiPrefix, apiKey string) ([]string, error) {
		attempts++
		return nil, errors.New("probe failed")
	}

	logServeStartupInfo(context.Background(), logger, "http://127.0.0.1:8080/v1", "fixed-key", discover, 2, 0)

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}

	out := buf.String()
	if !strings.Contains(out, `"msg":"gateway startup model discovery failed"`) {
		t.Fatalf("expected warning log for discovery failure, got: %s", out)
	}

	if !strings.Contains(out, `"error":"probe failed"`) {
		t.Fatalf("expected error detail in warning log, got: %s", out)
	}
}

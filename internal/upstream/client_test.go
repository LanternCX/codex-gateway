package upstream

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClient_WithHTTPClient_UsesInjectedClient(t *testing.T) {
	t.Parallel()

	called := false
	injected := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			if req.URL.String() != "https://example.com/v1/chat" {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := NewClient("https://example.com", 5*time.Second, WithHTTPClient(injected))

	resp, err := client.Do(context.Background(), http.MethodPost, "/v1/chat", []byte("{}"), "application/json", "token", nil)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if !called {
		t.Fatal("expected injected client transport to be called")
	}
	if client.httpClient != injected {
		t.Fatal("expected NewClient to use injected http client")
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
}

func TestClient_NewClient_WithoutHTTPClientOption_KeepsDefaultBehavior(t *testing.T) {
	t.Parallel()

	client := NewClient("https://example.com/", 5*time.Second)

	if client.httpClient == nil {
		t.Fatal("expected default http client to be initialized")
	}
	if client.httpClient.Timeout != 5*time.Second {
		t.Fatalf("expected timeout 5s, got %s", client.httpClient.Timeout)
	}
	if client.baseURL != "https://example.com" {
		t.Fatalf("expected trimmed base URL, got %q", client.baseURL)
	}
}

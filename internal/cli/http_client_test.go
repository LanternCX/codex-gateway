package cli

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClient_NoProxyURLUsesDefaultTransport(t *testing.T) {
	timeout := 5 * time.Second

	client, err := newHTTPClient(timeout, "   ")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if client == nil {
		t.Fatal("expected client, got nil")
	}

	if client.Timeout != timeout {
		t.Fatalf("unexpected timeout: got %v, want %v", client.Timeout, timeout)
	}

	if client.Transport != nil {
		t.Fatalf("expected default transport (nil), got %T", client.Transport)
	}
}

func TestNewHTTPClient_WithProxyURLSetsClonedDefaultTransportProxy(t *testing.T) {
	timeout := 7 * time.Second
	proxyURL := "http://proxy.example.com:8080"

	client, err := newHTTPClient(timeout, proxyURL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if client == nil {
		t.Fatal("expected client, got nil")
	}

	if client.Timeout != timeout {
		t.Fatalf("unexpected timeout: got %v, want %v", client.Timeout, timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}

	if transport == http.DefaultTransport {
		t.Fatal("expected cloned default transport, got http.DefaultTransport")
	}

	requestURL, err := url.Parse("https://api.example.com/v1/chat/completions")
	if err != nil {
		t.Fatalf("parse request url: %v", err)
	}

	req := &http.Request{URL: requestURL}
	selectedProxy, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("proxy func returned error: %v", err)
	}

	if selectedProxy == nil {
		t.Fatal("expected proxy url, got nil")
	}

	if selectedProxy.String() != proxyURL {
		t.Fatalf("unexpected proxy url: got %q, want %q", selectedProxy.String(), proxyURL)
	}
}

func TestNewHTTPClient_InvalidProxyURLReturnsError(t *testing.T) {
	_, err := newHTTPClient(3*time.Second, "localhost:8080")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "network.proxy_url") {
		t.Fatalf("expected error containing network.proxy_url, got: %v", err)
	}
}

func TestNewHTTPClient_UnsupportedProxySchemeReturnsError(t *testing.T) {
	_, err := newHTTPClient(3*time.Second, "ftp://proxy.example.com:21")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "network.proxy_url") {
		t.Fatalf("expected error containing network.proxy_url, got: %v", err)
	}
}

func TestNewHTTPClient_AbsoluteProxyURLWithoutHostReturnsError(t *testing.T) {
	_, err := newHTTPClient(3*time.Second, "http:///proxy")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "network.proxy_url") {
		t.Fatalf("expected error containing network.proxy_url, got: %v", err)
	}
}

func TestNewHTTPClient_ProxyURLWithoutHostnameReturnsError(t *testing.T) {
	_, err := newHTTPClient(3*time.Second, "http://:8080")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "network.proxy_url") {
		t.Fatalf("expected error containing network.proxy_url, got: %v", err)
	}
}

func TestNewHTTPClient_TrimmedProxyURLIsAccepted(t *testing.T) {
	timeout := 4 * time.Second
	proxyURL := "  http://proxy.example.com:8080  "

	client, err := newHTTPClient(timeout, proxyURL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}

	requestURL, err := url.Parse("https://api.example.com/v1/chat/completions")
	if err != nil {
		t.Fatalf("parse request url: %v", err)
	}

	req := &http.Request{URL: requestURL}
	selectedProxy, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("proxy func returned error: %v", err)
	}

	if selectedProxy == nil {
		t.Fatal("expected proxy url, got nil")
	}

	if selectedProxy.String() != strings.TrimSpace(proxyURL) {
		t.Fatalf("unexpected proxy url: got %q, want %q", selectedProxy.String(), strings.TrimSpace(proxyURL))
	}
}

func TestNewHTTPClient_ParseFailureWrapsURLError(t *testing.T) {
	_, err := newHTTPClient(3*time.Second, "http://[::1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "network.proxy_url") {
		t.Fatalf("expected error containing network.proxy_url, got: %v", err)
	}

	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Fatalf("expected wrapped *url.Error, got: %T (%v)", err, err)
	}
}

func TestNewOAuthHTTPClient_UsesThirtySecondTimeout(t *testing.T) {
	client, err := newOAuthHTTPClient("")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if client == nil {
		t.Fatal("expected client, got nil")
	}

	if client.Timeout != 30*time.Second {
		t.Fatalf("unexpected timeout: got %v, want %v", client.Timeout, 30*time.Second)
	}
}

func TestNewUpstreamHTTPClient_UsesConfiguredTimeout(t *testing.T) {
	client, err := newUpstreamHTTPClient(15, "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if client == nil {
		t.Fatal("expected client, got nil")
	}

	if client.Timeout != 15*time.Second {
		t.Fatalf("unexpected timeout: got %v, want %v", client.Timeout, 15*time.Second)
	}
}

func TestNewUpstreamHTTPClient_NonPositiveTimeoutDefaultsToSixtySeconds(t *testing.T) {
	client, err := newUpstreamHTTPClient(0, "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if client == nil {
		t.Fatal("expected client, got nil")
	}

	if client.Timeout != 60*time.Second {
		t.Fatalf("unexpected timeout: got %v, want %v", client.Timeout, 60*time.Second)
	}
}

package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func newHTTPClient(timeout time.Duration, proxyURL string) (*http.Client, error) {
	client := &http.Client{Timeout: timeout}

	trimmedProxyURL := strings.TrimSpace(proxyURL)
	if trimmedProxyURL == "" {
		return client, nil
	}

	parsedProxyURL, err := url.Parse(trimmedProxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid network.proxy_url %q (expected absolute URL with host): %w", proxyURL, err)
	}

	if !parsedProxyURL.IsAbs() || strings.TrimSpace(parsedProxyURL.Hostname()) == "" {
		return nil, fmt.Errorf("invalid network.proxy_url %q (expected absolute URL with host)", proxyURL)
	}

	proxyScheme := strings.ToLower(strings.TrimSpace(parsedProxyURL.Scheme))
	switch proxyScheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("invalid network.proxy_url %q (expected scheme http, https, socks5, or socks5h)", proxyURL)
	}

	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default transport type %T is not *http.Transport", http.DefaultTransport)
	}

	transport := baseTransport.Clone()
	transport.Proxy = http.ProxyURL(parsedProxyURL)
	client.Transport = transport

	return client, nil
}

func newOAuthHTTPClient(proxyURL string) (*http.Client, error) {
	return newHTTPClient(30*time.Second, proxyURL)
}

func newUpstreamHTTPClient(timeoutSeconds int, proxyURL string) (*http.Client, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}

	return newHTTPClient(time.Duration(timeoutSeconds)*time.Second, proxyURL)
}

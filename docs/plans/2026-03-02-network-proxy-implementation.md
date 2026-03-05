# Network Proxy Configuration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add explicit outbound proxy configuration so operators can actively set a proxy and have it applied to all gateway outbound HTTP requests (OAuth login/refresh and upstream API forwarding).

**Architecture:** Extend runtime config with an optional `network.proxy_url`, then construct proxy-aware HTTP clients in `internal/cli` and inject them into OAuth and upstream clients. Keep existing behavior unchanged when proxy is unset, and fail early with actionable config errors when proxy URL is invalid.

**Tech Stack:** Go 1.26, `net/http`, `net/url`, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, `httptest`, `go test`.

---

Execution constraints:
- Follow `@superpowers/test-driven-development` in every task.
- Follow repository Go conventions from `@code-standard`.

### Task 1: Add proxy config model and validation

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write the failing tests**

Add tests in `internal/config/config_test.go`:

```go
func TestLoad_ValidNetworkProxyURL(t *testing.T) {
	// config includes:
	// network:
	//   proxy_url: "http://127.0.0.1:7890"
	// Expect: Load succeeds and cfg.Network.ProxyURL is preserved.
}

func TestLoad_InvalidNetworkProxyURL(t *testing.T) {
	// config includes:
	// network:
	//   proxy_url: "://bad"
	// Expect: Load fails and error contains "network.proxy_url".
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config -run 'TestLoad_(ValidNetworkProxyURL|InvalidNetworkProxyURL)' -v`

Expected: FAIL because `Config` has no `Network` block and no validation.

**Step 3: Write the minimal implementation**

In `internal/config/config.go`:

```go
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
	OAuth    OAuthConfig    `yaml:"oauth"`
	Upstream UpstreamConfig `yaml:"upstream"`
	Network  NetworkConfig  `yaml:"network"`
}

type NetworkConfig struct {
	ProxyURL string `yaml:"proxy_url"`
}

if strings.TrimSpace(c.Network.ProxyURL) != "" {
	u, err := url.Parse(strings.TrimSpace(c.Network.ProxyURL))
	if err != nil || !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("invalid network.proxy_url %q", c.Network.ProxyURL)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config -run 'TestLoad_(ValidNetworkProxyURL|InvalidNetworkProxyURL)' -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add network proxy config validation"
```

### Task 2: Build proxy-aware HTTP client helpers in CLI

**Files:**
- Create: `internal/cli/http_client.go`
- Create: `internal/cli/http_client_test.go`

**Step 1: Write the failing tests**

Create tests in `internal/cli/http_client_test.go`:

```go
func TestNewHTTPClient_NoProxy(t *testing.T) {
	client, err := newHTTPClient(10*time.Second, "")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if client.Timeout != 10*time.Second { t.Fatalf("timeout mismatch") }
}

func TestNewHTTPClient_WithProxy(t *testing.T) {
	client, err := newHTTPClient(10*time.Second, "http://127.0.0.1:7890")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	t, _ := client.Transport.(*http.Transport)
	if t == nil || t.Proxy == nil { t.Fatalf("expected proxy transport") }
}

func TestNewHTTPClient_InvalidProxy(t *testing.T) {
	_, err := newHTTPClient(10*time.Second, "://bad")
	if err == nil { t.Fatal("expected error") }
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli -run 'TestNewHTTPClient' -v`

Expected: FAIL because helper does not exist.

**Step 3: Write the minimal implementation**

Create `internal/cli/http_client.go`:

```go
func newHTTPClient(timeout time.Duration, proxyURL string) (*http.Client, error) {
	client := &http.Client{Timeout: timeout}
	if strings.TrimSpace(proxyURL) == "" {
		return client, nil
	}
	u, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil || !u.IsAbs() || u.Host == "" {
		return nil, fmt.Errorf("invalid network.proxy_url %q", proxyURL)
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = http.ProxyURL(u)
	client.Transport = t
	return client, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli -run 'TestNewHTTPClient' -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/http_client.go internal/cli/http_client_test.go
git commit -m "feat: add proxy-aware outbound http client builder"
```

### Task 3: Add injectable HTTP client option to upstream client

**Files:**
- Modify: `internal/upstream/client.go`
- Create: `internal/upstream/client_test.go`

**Step 1: Write the failing test**

Create `internal/upstream/client_test.go`:

```go
func TestClient_WithHTTPClient_UsesInjectedClient(t *testing.T) {
	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.example.com/v1/models" {
			t.Fatalf("unexpected url: %s", req.URL.String())
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
	})

	injected := &http.Client{Transport: rt, Timeout: 2 * time.Second}
	c := NewClient("https://api.example.com", 30*time.Second, WithHTTPClient(injected))
	resp, err := c.Do(context.Background(), http.MethodGet, "/v1/models", nil, "", "token", nil)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	resp.Body.Close()
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/upstream -run TestClient_WithHTTPClient_UsesInjectedClient -v`

Expected: FAIL because `WithHTTPClient` option does not exist.

**Step 3: Write minimal implementation**

In `internal/upstream/client.go` add:

```go
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/upstream -run TestClient_WithHTTPClient_UsesInjectedClient -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/upstream/client.go internal/upstream/client_test.go
git commit -m "feat: allow injected upstream http client"
```

### Task 4: Wire proxy-aware clients into auth login and server startup

**Files:**
- Modify: `internal/cli/auth_login.go`
- Modify: `internal/cli/serve.go`
- Modify: `internal/cli/http_client.go`
- Modify: `internal/cli/http_client_test.go`

**Step 1: Write the failing tests**

Add tests in `internal/cli/http_client_test.go` for component-specific builders:

```go
func TestNewOAuthHTTPClient_UsesOAuthTimeout(t *testing.T) {
	client, err := newOAuthHTTPClient("http://127.0.0.1:7890")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if client.Timeout != 30*time.Second { t.Fatalf("unexpected oauth timeout") }
}

func TestNewUpstreamHTTPClient_UsesConfiguredTimeout(t *testing.T) {
	client, err := newUpstreamHTTPClient(45, "http://127.0.0.1:7890")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if client.Timeout != 45*time.Second { t.Fatalf("unexpected upstream timeout") }
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli -run 'TestNew(OAuth|Upstream)HTTPClient' -v`

Expected: FAIL because builders do not exist.

**Step 3: Write minimal implementation**

In `internal/cli/http_client.go`, add builders:

```go
func newOAuthHTTPClient(proxyURL string) (*http.Client, error) {
	return newHTTPClient(30*time.Second, proxyURL)
}

func newUpstreamHTTPClient(timeoutSeconds int, proxyURL string) (*http.Client, error) {
	t := time.Duration(timeoutSeconds) * time.Second
	if t <= 0 {
		t = 60 * time.Second
	}
	return newHTTPClient(t, proxyURL)
}
```

Then wire clients:
- `internal/cli/auth_login.go`: call `newOAuthHTTPClient(cfg.Network.ProxyURL)` and pass `oauth.WithHTTPClient(client)`.
- `internal/cli/serve.go`: call both builders and pass `oauth.WithHTTPClient(...)` and `upstream.WithHTTPClient(...)`.

**Step 4: Run tests to verify pass and no regressions**

Run:
- `go test ./internal/cli -run 'TestNew(HTTPClient|OAuthHTTPClient|UpstreamHTTPClient)' -v`
- `go test ./internal/oauth -run TestClient_AuthenticateSuccess -v`
- `go test ./internal/server -run TestProxy -v`

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/http_client.go internal/cli/http_client_test.go internal/cli/auth_login.go internal/cli/serve.go
git commit -m "feat: apply configured proxy to oauth and upstream flows"
```

### Task 5: Update sample config and user docs

**Files:**
- Modify: `config.example.yaml`
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/en/oauth-setup.md`
- Modify: `docs/zh-CN/oauth-setup.md`
- Modify: `docs/en/README.md`
- Modify: `docs/zh-CN/README.md`

**Step 1: Write docs expectations checklist (failing manual check)**

Document the checklist in your working notes:
- example config shows where to set proxy
- English/Chinese quick start mention proxy behavior
- OAuth setup docs mention proxy applies to OAuth + upstream requests

**Step 2: Implement documentation updates**

Update docs with a short config snippet:

```yaml
network:
  proxy_url: "http://127.0.0.1:7890"
```

Clarify that empty value means no explicit proxy.

**Step 3: Run lightweight docs verification**

Run: `go test ./internal/config -run 'TestLoad_(ValidNetworkProxyURL|InvalidNetworkProxyURL)' -v`

Expected: PASS (docs match actual config behavior).

**Step 4: Commit**

```bash
git add config.example.yaml README.md README.zh-CN.md docs/en/oauth-setup.md docs/zh-CN/oauth-setup.md docs/en/README.md docs/zh-CN/README.md
git commit -m "docs: describe configurable outbound proxy"
```

### Task 6: Full verification

**Files:**
- Modify: none expected

**Step 1: Run full test suite**

Run: `go test ./...`

Expected: PASS.

**Step 2: Run race tests**

Run: `go test -race ./...`

Expected: PASS.

**Step 3: Run build check**

Run: `go build ./cmd/codex-gateway`

Expected: PASS.

**Step 4: Commit if verification introduced fixes**

```bash
git add <any-fixed-files>
git commit -m "chore: fix verification follow-ups for proxy feature"
```

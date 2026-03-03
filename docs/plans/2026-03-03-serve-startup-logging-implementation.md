# Serve Startup Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Print API prefix and discovered models after `serve` starts, and make default logs human-readable with terminal colors.

**Architecture:** Keep startup metadata logic inside `internal/cli` so `internal/server` remains focused on transport. Add small pure helpers for API prefix rendering and model discovery, then call them from `runServe` as a non-blocking best-effort startup probe. Preserve existing request handling and shutdown flow.

**Tech Stack:** Go (`net/http`, `encoding/json`, `cobra`, `slog`), existing config loader/tests, repository markdown docs.

---

### Task 1: Add API Prefix Helper With TDD

**Files:**
- Create: `internal/cli/startup_probe.go`
- Create: `internal/cli/startup_probe_test.go`

**Step 1: Write the failing test**

```go
func TestBuildAPIPrefix(t *testing.T) {
    tests := []struct {
        name   string
        listen string
        want   string
    }{
        {name: "port only", listen: ":8080", want: "http://127.0.0.1:8080/v1"},
        {name: "wildcard host", listen: "0.0.0.0:9000", want: "http://127.0.0.1:9000/v1"},
        {name: "named host", listen: "localhost:7777", want: "http://localhost:7777/v1"},
    }
    // ...assert buildAPIPrefix(tc.listen) == tc.want
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestBuildAPIPrefix -v`
Expected: FAIL because helper does not exist.

**Step 3: Write minimal implementation**

```go
func buildAPIPrefix(listen string) string {
    host, port := splitListen(listen)
    return "http://" + net.JoinHostPort(host, port) + "/v1"
}
```

Rules:
- Convert empty/wildcard hosts (`""`, `"0.0.0.0"`, `"::"`) to `127.0.0.1`.
- Keep explicit host values unchanged.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestBuildAPIPrefix -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/startup_probe.go internal/cli/startup_probe_test.go
git commit -m "feat: add api prefix helper for serve startup logs"
```

### Task 2: Add Model Discovery Probe With TDD

**Files:**
- Modify: `internal/cli/startup_probe.go`
- Modify: `internal/cli/startup_probe_test.go`

**Step 1: Write failing tests for discovery behavior**

```go
func TestDiscoverAvailableModels_Success(t *testing.T) {
    // httptest server returns {"data":[{"id":"gpt-5.3-codex"},{"id":"gpt-5.2-codex"}]}
    // expect []string{"gpt-5.3-codex", "gpt-5.2-codex"}
}

func TestDiscoverAvailableModels_HTTPError(t *testing.T) {
    // server returns 401; expect error
}

func TestDiscoverAvailableModels_InvalidPayload(t *testing.T) {
    // invalid JSON; expect error
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli -run TestDiscoverAvailableModels -v`
Expected: FAIL because function does not exist.

**Step 3: Implement minimal discovery function**

```go
func discoverAvailableModels(ctx context.Context, client *http.Client, apiPrefix, apiKey string) ([]string, error) {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiPrefix+"/models", nil)
    req.Header.Set("Authorization", "Bearer "+apiKey)
    // execute request, decode JSON, extract non-empty id values
}
```

Rules:
- Return error for non-2xx responses.
- Deduplicate IDs while keeping first-seen order.
- Ignore empty IDs.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli -run TestDiscoverAvailableModels -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/startup_probe.go internal/cli/startup_probe_test.go
git commit -m "feat: add serve startup model discovery probe"
```

### Task 3: Integrate Startup Logs Into `runServe`

**Files:**
- Modify: `internal/cli/serve.go`
- Modify: `internal/cli/startup_probe.go`
- Modify: `internal/cli/startup_probe_test.go`

**Step 1: Write failing integration-level unit test for startup logging helper**

```go
func TestLogServeStartupInfo_LogsPrefixAndModels(t *testing.T) {
    // use buffer-backed logger
    // call helper with fake discover func returning models
    // assert log contains api_prefix and available_models
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestLogServeStartupInfo -v`
Expected: FAIL because helper is not wired.

**Step 3: Implement wiring in `runServe`**

```go
apiPrefix := buildAPIPrefix(cfg.Server.Listen)
logger.InfoContext(ctx, "gateway server starting", "listen", cfg.Server.Listen, "workdir", paths.Workdir, "upstream_mode", cfg.Upstream.Mode, "api_prefix", apiPrefix)

go logServeStartupInfo(ctx, logger, apiPrefix, cfg.Auth.DownstreamAPIKey)
```

`logServeStartupInfo` rules:
- short timeout (for example 3s)
- best-effort retries for early startup race (small sleep between attempts)
- warning log on final failure only
- info log with `available_models` on success

**Step 4: Run relevant tests**

Run: `go test ./internal/cli -run "Test(BuildAPIPrefix|DiscoverAvailableModels|LogServeStartupInfo)" -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/serve.go internal/cli/startup_probe.go internal/cli/startup_probe_test.go
git commit -m "feat: log api prefix and available models on serve startup"
```

### Task 4: Switch Default Log Experience to Human-Friendly + Update Docs

**Files:**
- Modify: `config.example.yaml`
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/en/oauth-setup.md`
- Modify: `docs/zh-CN/oauth-setup.md`
- Test: `internal/config/config_test.go`

**Step 1: Add/adjust failing config default assertions**

```go
func TestLoad_LoggingDefaults(t *testing.T) {
    // assert cfg.Logging.Format == "text"
    // assert cfg.Logging.Color == "auto"
}
```

**Step 2: Run test to verify current expectation behavior**

Run: `go test ./internal/config -run TestLoad_LoggingDefaults -v`
Expected: PASS for code defaults; then align sample/docs.

**Step 3: Implement doc/config updates**

- Set `logging.format: "text"` in `config.example.yaml`.
- Update README EN/ZH startup section to mention startup logs include `api_prefix` and `available_models`.
- Update OAuth setup docs EN/ZH to state text+auto is the default human-friendly logging mode.

**Step 4: Run package tests and full verification**

Run:
- `go test ./internal/cli ./internal/config ./internal/logging`
- `go test ./...`
- `go test -race ./...`
- `go build ./cmd/codex-gateway`

Expected: all PASS.

**Step 5: Commit**

```bash
git add config.example.yaml README.md README.zh-CN.md docs/en/oauth-setup.md docs/zh-CN/oauth-setup.md internal/config/config_test.go
git commit -m "docs: default serve logs to human-readable text output"
```

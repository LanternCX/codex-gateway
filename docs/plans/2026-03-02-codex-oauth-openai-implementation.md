# Codex OAuth OpenAI Gateway Implementation Plan

Docs: [Index](../en/README.md) · [文档索引](../zh-CN/README.md) · [Design](2026-03-02-codex-oauth-openai-design.md)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Deliver a production-usable Go gateway that authenticates upstream calls via Codex OAuth while exposing `/v1/models` and `/v1/chat/completions` in OpenAI-compatible format with a fixed downstream API key.

**Architecture:** Build a single Go binary with CLI subcommands for OAuth login and HTTP serving. Keep runtime artifacts (`config.yaml`, `oauth-token.json`) in the working directory and route requests through a token manager that auto-refreshes OAuth credentials. Implement OpenAI-style error responses and transparent streaming pass-through.

**Tech Stack:** Go 1.26, `net/http`, `encoding/json`, `gopkg.in/yaml.v3`, `github.com/spf13/cobra`, `httptest`, `go test`.

---

### Task 1: Bootstrap module and command entrypoint

**Files:**
- Create: `go.mod`
- Create: `cmd/codex-gateway/main.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/serve.go`
- Create: `internal/cli/auth_login.go`

**Step 1: Write the failing smoke test**

Create `internal/cli/root_test.go` asserting root command has `serve` and `auth login` subcommands.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestRootCommand_Subcommands -v`
Expected: FAIL due missing command implementation.

**Step 3: Write minimal implementation**

Implement Cobra root command and add subcommands.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestRootCommand_Subcommands -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add go.mod cmd/codex-gateway/main.go internal/cli
git commit -m "feat: bootstrap codex-gateway cli entrypoint"
```

### Task 2: Implement runtime config loading and validation

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Step 1: Write the failing tests**

Add tests for:
- missing required fields (`server.listen`, `auth.downstream_api_key`, OAuth/token endpoints)
- valid config parses successfully

**Step 2: Run tests to verify failure**

Run: `go test ./internal/config -v`
Expected: FAIL with undefined loader/validator.

**Step 3: Write minimal implementation**

Implement YAML loader and validation function.

**Step 4: Run tests to verify pass**

Run: `go test ./internal/config -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/config
git commit -m "feat: add runtime config loader and validation"
```

### Task 3: Build OAuth token store and token manager

**Files:**
- Create: `internal/auth/token_store.go`
- Create: `internal/auth/manager.go`
- Test: `internal/auth/token_store_test.go`
- Test: `internal/auth/manager_test.go`

**Step 1: Write failing tests**

Cover:
- save/load token JSON in runtime directory
- token expiry detection with refresh buffer
- refresh failure path returns actionable error

**Step 2: Run tests to verify failure**

Run: `go test ./internal/auth -run TestTokenStore -v`
Expected: FAIL due missing store implementation.

**Step 3: Write minimal implementation**

Implement token file persistence and manager with refresh callback.

**Step 4: Run tests to verify pass**

Run: `go test ./internal/auth -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/auth
git commit -m "feat: add oauth token store and refresh manager"
```

### Task 4: Add OAuth device-flow login command

**Files:**
- Create: `internal/oauth/device_flow.go`
- Modify: `internal/cli/auth_login.go`
- Test: `internal/oauth/device_flow_test.go`

**Step 1: Write failing tests**

Add fake OAuth server tests for:
- successful device code + polling path
- terminal errors (`access_denied`, `expired_token`)

**Step 2: Run tests to verify failure**

Run: `go test ./internal/oauth -v`
Expected: FAIL due missing flow implementation.

**Step 3: Write minimal implementation**

Implement:
- request device code
- print verification instructions
- poll token endpoint until success/failure
- persist token to store

**Step 4: Run tests to verify pass**

Run: `go test ./internal/oauth -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/oauth internal/cli/auth_login.go
git commit -m "feat: implement oauth device login command"
```

### Task 5: Implement downstream auth middleware and error envelope

**Files:**
- Create: `internal/server/errors.go`
- Create: `internal/server/middleware_auth.go`
- Test: `internal/server/middleware_auth_test.go`

**Step 1: Write failing tests**

Add tests for missing/invalid/valid downstream bearer key.

**Step 2: Run tests to verify failure**

Run: `go test ./internal/server -run TestAuthMiddleware -v`
Expected: FAIL due missing middleware.

**Step 3: Write minimal implementation**

Implement middleware and OpenAI-style error response helper.

**Step 4: Run tests to verify pass**

Run: `go test ./internal/server -run TestAuthMiddleware -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/server/errors.go internal/server/middleware_auth.go internal/server/middleware_auth_test.go
git commit -m "feat: enforce fixed downstream api key auth"
```

### Task 6: Implement upstream proxy client for models and chat completions

**Files:**
- Create: `internal/upstream/client.go`
- Create: `internal/server/handlers.go`
- Create: `internal/server/server.go`
- Modify: `internal/cli/serve.go`
- Test: `internal/server/handlers_test.go`

**Step 1: Write failing tests**

Use `httptest` upstream to verify:
- `/v1/models` forwards with OAuth bearer
- `/v1/chat/completions` forwards request body
- streaming response headers/body are relayed

**Step 2: Run tests to verify failure**

Run: `go test ./internal/server -run TestProxy -v`
Expected: FAIL due missing handlers/client.

**Step 3: Write minimal implementation**

Implement proxy handlers and retry-once behavior for expired upstream token after refresh.

**Step 4: Run tests to verify pass**

Run: `go test ./internal/server -run TestProxy -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/upstream internal/server internal/cli/serve.go
git commit -m "feat: add openai-compatible proxy endpoints"
```

### Task 7: Add health endpoint and end-to-end server test

**Files:**
- Modify: `internal/server/server.go`
- Test: `internal/server/server_test.go`

**Step 1: Write failing tests**

Add tests for:
- `GET /healthz` returns 200
- auth-protected routes reject missing key

**Step 2: Run tests to verify failure**

Run: `go test ./internal/server -run TestServer -v`
Expected: FAIL if route wiring is incomplete.

**Step 3: Write minimal implementation**

Wire health and protected routes with middleware chain.

**Step 4: Run tests to verify pass**

Run: `go test ./internal/server -run TestServer -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/server
git commit -m "feat: wire health and protected route groups"
```

### Task 8: Docs, example config, and full verification

**Files:**
- Create: `config.example.yaml`
- Create: [README.md](../../README.md)

**Step 1: Write failing docs sanity check (manual)**

Try commands from draft README on a clean shell; note missing details.

**Step 2: Add documentation and sample config**

Document login flow, serve command, runtime directory semantics, endpoint examples.

**Step 3: Run full verification**

Run:
- `go test ./...`
- `go test -race ./...`

Expected: PASS.

**Step 4: Final commit**

```bash
git add README.md config.example.yaml
git commit -m "docs: add usage guide and runtime config example"
```

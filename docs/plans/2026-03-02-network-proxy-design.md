# Configurable Outbound Proxy Design

Docs: [Index](../en/README.md) · [文档索引](../zh-CN/README.md) · [Implementation Plan](2026-03-02-network-proxy-implementation.md)

## Goal

Allow operators to explicitly configure an outbound proxy so that all gateway-initiated HTTP traffic can be routed through a chosen proxy endpoint.

## Scope

- Add explicit proxy configuration in `config.yaml`.
- Apply proxy setting to all outbound requests:
  - OAuth login token exchange requests
  - OAuth refresh requests
  - Upstream API proxy requests (`/v1/models`, `/v1/chat/completions`)
- Keep behavior backward-compatible when proxy is not configured.

## Approaches Considered

1. **Config-driven proxy (recommended)**
   - Add `network.proxy_url` and inject configured `http.Client` into OAuth and upstream clients.
   - Pros: explicit, deterministic, easy to document and reproduce.
   - Cons: requires moderate plumbing changes.

2. **CLI flag-driven proxy**
   - Add `--proxy` to `serve`/`auth login` and optionally override config.
   - Pros: convenient one-off usage.
   - Cons: more CLI/API surface and precedence complexity.

3. **Environment variables only**
   - Rely on `HTTP_PROXY`/`HTTPS_PROXY`.
   - Pros: no code/config changes.
   - Cons: not explicit in gateway config; weaker operability and repeatability.

## Chosen Design

Use a config-driven approach with explicit proxy injection.

### Configuration

- Add `network` block to config model:
  - `network.proxy_url` (optional)
- Validation rules:
  - Empty value is allowed (proxy disabled).
  - Non-empty value must be a valid absolute URL parseable by `net/url`.

### Client Construction

- Add a shared CLI helper to create HTTP clients with optional proxy.
- Preserve per-component timeout behavior:
  - OAuth client requests continue using OAuth-oriented timeout.
  - Upstream client requests continue using `upstream.timeout_seconds`.

### Dependency Wiring

- OAuth package already supports `WithHTTPClient`; wire it in both:
  - `internal/cli/auth_login.go`
  - `internal/cli/serve.go`
- Upstream package gains `WithHTTPClient` option and uses injected client when provided.

### Runtime Behavior

- If `network.proxy_url` is set, all outbound HTTP requests use that proxy.
- If unset, keep existing behavior unchanged.

## Error Handling

- Invalid `network.proxy_url` fails config load with actionable field-specific error.
- Proxy connection failures surface as existing upstream/OAuth request errors with wrapped context.

## Testing Strategy

- `internal/config`:
  - valid `network.proxy_url` accepted
  - invalid `network.proxy_url` rejected
- `internal/upstream`:
  - injected HTTP client is used by request path
- `internal/cli`:
  - helper builds proxied transport when proxy URL is configured

## Documentation Changes

- Update `config.example.yaml` with `network.proxy_url`.
- Update user docs describing how to enable outbound proxy:
  - `README.md`
  - `README.zh-CN.md`
  - `docs/en/oauth-setup.md`
  - `docs/zh-CN/oauth-setup.md`

## Non-Goals

- Per-route or per-upstream proxy selection.
- Authenticated proxy credential management UX beyond URL-embedded credentials.
- Dynamic proxy reload without process restart.

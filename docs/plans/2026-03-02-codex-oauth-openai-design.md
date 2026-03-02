# Codex OAuth to OpenAI-Compatible Gateway Design

Docs: [Index](../en/README.md) · [文档索引](../zh-CN/README.md) · [Implementation Plan](2026-03-02-codex-oauth-openai-implementation.md)

## Goal

Build a self-hosted gateway that accepts OpenAI-style API requests from downstream clients while authenticating upstream requests with Codex OAuth tokens. Downstream access control uses a single fixed API key stored in runtime configuration.

## Scope (v1)

- Implement interactive CLI login for Codex OAuth.
- Store all runtime artifacts in the service run directory.
- Expose OpenAI-compatible endpoints:
  - `GET /v1/models`
  - `POST /v1/chat/completions` (including streaming pass-through)
- Validate downstream `Authorization: Bearer <fixed_key>` from config.

## Runtime Directory Layout

All files are rooted in process workdir (`cwd`) unless `--workdir` overrides it.

- `./config.yaml`: gateway config, fixed downstream key, upstream URL, OAuth endpoints.
- `./oauth-token.json`: OAuth access/refresh token cache.
- `./logs/` (optional): structured logs.

## Architecture

Single Go binary with two command groups:

- `codex-gateway auth login`: runs interactive OAuth device flow and saves tokens.
- `codex-gateway serve`: starts HTTP server with OpenAI-compatible API.

Core components:

- Config loader: reads and validates `config.yaml`.
- Token store: reads/writes token JSON in runtime directory.
- OAuth client: device authorization + token polling + refresh.
- Token manager: ensures valid access token before upstream calls.
- Upstream client: sends proxied OpenAI requests to upstream base URL.
- HTTP server: auth middleware, endpoints, health check, error mapping.

## Data Flows

### Login Flow

1. Operator runs `codex-gateway auth login`.
2. CLI requests a device code from OAuth device authorization endpoint.
3. CLI prints verification URL and user code for browser-based confirmation.
4. CLI polls token endpoint until success or terminal error.
5. CLI writes token data to `./oauth-token.json`.

### API Request Flow

1. Client calls gateway with OpenAI-compatible request + fixed bearer key.
2. Middleware checks bearer token against configured fixed key.
3. Handler asks token manager for valid upstream access token.
4. Gateway forwards request to upstream endpoint with OAuth bearer token.
5. Gateway relays response body/status to client (JSON or SSE stream).

### Refresh Flow

1. Before each upstream call, token manager checks expiration buffer.
2. If near expiry, it refreshes via OAuth token endpoint.
3. On success, refreshed token is persisted atomically.
4. On refresh failure, request returns actionable error requiring `auth login`.

## Error Model

- `401 Unauthorized`: downstream fixed API key missing/invalid.
- `503 Service Unavailable`: OAuth token absent or refresh failed.
- `502 Bad Gateway`: upstream request failed or returned unusable response.

All error responses follow OpenAI-style shape:

```json
{
  "error": {
    "message": "human readable",
    "type": "gateway_error",
    "code": "string_code"
  }
}
```

## Security Notes

- Token file permissions should be user-only when possible.
- Never log access_token, refresh_token, or downstream fixed API key.
- Keep config/token in private server directory with restricted ownership.

## Testing Strategy

- Unit tests for config validation and token persistence.
- Unit tests for auth middleware behavior.
- HTTP integration tests using `httptest` for proxy endpoints.
- OAuth refresh tests with fake OAuth server endpoints.

## Non-Goals (v1)

- Full OpenAI surface (embeddings/audio/images/fine-tuning).
- Multi-tenant key management.
- Distributed token storage.

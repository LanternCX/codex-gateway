# codex-gateway Architecture

Language: [English](architecture.md) | [简体中文](../zh-CN/architecture.md)

Docs: [Index](README.md) · [API Reference](api-reference.md) · [OpenAPI Spec](../openapi.yaml) · [OAuth Setup](oauth-setup.md) · [Privacy Boundary](privacy-boundary.md) · [Git Flow](git-flow.md)

## Overview

`codex-gateway` is a single Go binary that provides:

- interactive OAuth login via CLI (`auth login`)
  - callback mode (default, local browser redirect)
- OpenAI-compatible HTTP endpoints (`/v1/models`, `/v1/chat/completions`)
- fixed downstream API-key authentication

The gateway authenticates upstream requests using OAuth tokens stored in runtime directory files.

## Runtime Layout

By default, runtime artifacts are resolved from the current working directory (or `--workdir`):

- `config.yaml`: server/auth/oauth/upstream settings
- `oauth-token.json`: persisted OAuth access/refresh token
- structured logs: `stdout`, `file`, or `both` controlled by `logging.output`; text color mode controlled by `logging.color`
- when file logging is enabled, default log directory is `<workdir>/logs`

## Package Boundaries

- `cmd/codex-gateway`: executable entrypoint.
- `internal/cli`: Cobra command wiring (`serve`, `auth login`).
- `internal/config`: YAML config loading + validation.
- `internal/logging`: logger construction, redaction, request-id propagation helpers, and multi-sink output handling.
- `internal/auth`: token persistence + refresh policy manager.
- `internal/oauth`: OAuth device flow and refresh HTTP interactions.
- `internal/upstream`: upstream HTTP client wrapper.
- `internal/server`: downstream HTTP server, auth middleware, error mapping, proxy handlers.

## Request Flow

1. Downstream client calls `/v1/models` or `/v1/chat/completions` with `Authorization: Bearer <fixed-key>`.
2. `internal/server` validates fixed downstream API key.
3. Handler asks token manager for upstream access token.
4. If token is near expiry, manager refreshes via OAuth token endpoint and rewrites token file.
5. In `codex_oauth` mode, gateway transforms chat-completions into Codex responses backend format and converts results back to OpenAI chat-completions format.
6. In `openai_api` mode, gateway directly proxies upstream request/response.

`/v1/models` behavior:

- `codex_oauth` mode: returns a static compatibility list of Codex-capable model ids.
- `openai_api` mode: proxies upstream `/v1/models`.

## Error Mapping

- `401`: missing/invalid downstream fixed API key
- `503`: OAuth token missing/refresh failed
- `502`: upstream request failure or upstream 5xx mapped failure

Responses use OpenAI-style error envelope:

```json
{
  "error": {
    "message": "...",
    "type": "gateway_error",
    "code": "..."
  }
}
```

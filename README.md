# codex-gateway

Language: [English](README.md) | [简体中文](README.zh-CN.md)

Quick Links: [Features](#features) · [Runtime Directory](#runtime-directory) · [Quick Start](#quick-start) · [API Reference](#api-reference) · [Errors](#errors) · [Development](#development) · [Docs Index](docs/en/README.md)

A self-hosted gateway that:

- accepts OpenAI-compatible downstream requests (`/v1/models`, `/v1/chat/completions`)
- authenticates upstream requests with OAuth tokens obtained via interactive CLI login
- protects downstream access using one fixed API key from config

## Features

- Interactive OAuth callback login (default): `codex-gateway auth login`
- Runtime directory storage (`config.yaml`, `oauth-token.json`)
- Default upstream mode is `codex_oauth` (compatible with ChatGPT OAuth tokens)
- OpenAI-compatible endpoints:
  - `GET /v1/models`
  - `POST /v1/chat/completions` (stream pass-through supported)
  - In `codex_oauth` mode, `/v1/models` returns a compatibility model list and `/v1/chat/completions` is translated to Codex responses backend.
- Fixed downstream API key validation via `Authorization: Bearer <fixed_key>`
- Automatic OAuth refresh before upstream calls
- Structured logging with configurable `logging.level` and `logging.format`
- Health endpoint: `GET /healthz`

## Documentation

- Documentation index: [docs/en/README.md](docs/en/README.md)
- Architecture: [docs/en/architecture.md](docs/en/architecture.md)
- API reference: [docs/en/api-reference.md](docs/en/api-reference.md)
- OpenAPI spec: [docs/openapi.yaml](docs/openapi.yaml)
- OAuth setup: [docs/en/oauth-setup.md](docs/en/oauth-setup.md)
- Privacy boundary: [docs/en/privacy-boundary.md](docs/en/privacy-boundary.md)
- Git workflow: [docs/en/git-flow.md](docs/en/git-flow.md)
- 中文文档入口: [docs/zh-CN/README.md](docs/zh-CN/README.md)

## Runtime Directory

All runtime files are resolved from `--workdir` (default: current directory):

- `config.yaml`
- `oauth-token.json`
- Structured logs emitted to stdout (`logging.level`, `logging.format`)

Runtime path policy:

- `--config` must point to a file inside `--workdir`
- gateway-generated runtime artifacts are stored under `--workdir` only

Upstream mode:

- `upstream.mode: codex_oauth` (default): transform chat-completions to Codex backend responses flow
- `upstream.mode: openai_api`: direct proxy to `upstream.base_url`

## Quick Start

1) Build:

```bash
go build -o codex-gateway ./cmd/codex-gateway
```

2) Prepare config:

```bash
cp config.example.yaml config.yaml
```

Then edit `config.yaml` with at least your fixed downstream key and upstream base URL.
For Codex OAuth callback mode, OAuth endpoints and client id already have defaults.

3) Run OAuth login (interactive):

```bash
./codex-gateway auth login --workdir . --config config.yaml
```

This command starts a local callback listener and opens the browser authorization URL.

4) Start server:

```bash
./codex-gateway serve --workdir . --config config.yaml
```

## API Reference

- Formal API reference: [docs/en/api-reference.md](docs/en/api-reference.md)
- Apifox-importable OpenAPI file: [docs/openapi.yaml](docs/openapi.yaml)

Endpoint summary:

- `GET /healthz`
- `GET /v1/models`
- `POST /v1/chat/completions`

Request payload example (`POST /v1/chat/completions`):

```json
{
  "model": "gpt-5.3-codex",
  "messages": [
    {
      "role": "user",
      "content": "Reply with exactly: hello"
    }
  ],
  "stream": false
}
```

## Errors

Gateway errors are returned in OpenAI-style envelope:

```json
{
  "error": {
    "message": "...",
    "type": "gateway_error",
    "code": "..."
  }
}
```

Common status mapping:

- `401`: downstream fixed API key missing/invalid
- `503`: OAuth token unavailable or refresh failed
- `502`: upstream network/service error

## Development

Run tests:

```bash
go test ./...
go test -race ./...
```

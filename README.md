# codex-gateway

Language: [English](README.md) | [简体中文](README.zh-CN.md)

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
- Structured logging with configurable level/format/output/color and file rotation settings
- Request correlation via `X-Request-ID` (auto-generated when missing)
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
- Structured logs emitted to stdout or file (`logging.output`)
- `logs/` when `logging.output` is `file` or `both` (default path `<workdir>/logs`)

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

2) Prepare config (from repository root):

```bash
cp config.example.yaml config.yaml
```

Then edit `config.yaml` with at least `auth.downstream_api_key`.
`upstream.mode` defaults to `codex_oauth` when omitted; set `upstream.base_url` only when `upstream.mode: openai_api`.
For Codex OAuth callback mode, OAuth endpoints and client id already have defaults.
If needed, you can set an outbound proxy for both `auth login` and `serve` requests:

```yaml
network:
  proxy_url: "http://127.0.0.1:7890"
```

`network.proxy_url` must be an absolute URL with host and a supported scheme: `http`, `https`, `socks5`, or `socks5h` (for example, `http://127.0.0.1:7890` or `socks5h://127.0.0.1:1080`).
Leave `network.proxy_url` empty or unset to use no explicit proxy.

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

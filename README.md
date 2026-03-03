# codex-gateway

Language: [English](README.md) | [简体中文](README.zh-CN.md)

A self-hosted gateway that:

- accepts OpenAI-compatible downstream requests (`/v1/models`, `/v1/chat/completions`, `/v1/responses`)
- authenticates upstream requests with OAuth tokens obtained via interactive CLI login
- protects downstream access using one fixed API key from config

## Features

- Interactive OAuth callback login (default): `codex-gateway auth login`
- Runtime directory storage (`config.yaml`, `oauth-token.json`)
- Default upstream mode is `codex_oauth` (compatible with ChatGPT OAuth tokens)
- OpenAI-compatible endpoints:
  - `GET /v1/models`
  - `POST /v1/chat/completions` (streaming supported)
  - `POST /v1/responses` (JSON and stream pass-through supported)
  - In `codex_oauth` mode, `/v1/models` returns a compatibility model list; `/v1/chat/completions` is transformed into Codex responses backend requests/results; `/v1/responses` proxies to Codex responses backend path (default `/backend-api/codex/responses`, configurable via `upstream.codex_responses_path`).
  - In `openai_api` mode, `/v1/chat/completions` and `/v1/responses` proxy to upstream paths.
- Fixed downstream API key validation via `Authorization: Bearer <fixed_key>`
- Automatic OAuth refresh before upstream calls
- Structured logging with configurable level/format/output/color and file rotation settings
- Default stdout logging is human-readable text with terminal color auto-detection
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

After startup, logs include:

- `api_prefix` (for example `http://127.0.0.1:8080/v1`)
- `available_models` discovered via a startup probe (`GET /v1/models`)

## API Reference

- Formal API reference: [docs/en/api-reference.md](docs/en/api-reference.md)
- Apifox-importable OpenAPI file: [docs/openapi.yaml](docs/openapi.yaml)

Endpoint summary:

- `GET /healthz`
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`

## OpenCode custom provider

For OpenCode clients targeting this gateway and expecting codex-like responses/thinking behavior, prefer `@ai-sdk/openai` as the custom provider package (instead of generic OpenAI-compatible adapters).

`opencode.json` example:

```json
{
  "providers": {
    "gateway": {
      "package": "@ai-sdk/openai",
      "name": "Gateway",
      "options": {
        "baseURL": "http://127.0.0.1:8080/v1",
        "apiKey": "<downstream_api_key>"
      }
    }
  },
  "models": {
    "gateway/gpt-5.3-codex": {
      "reasoning": true,
      "limit": {
        "input": 200000,
        "output": 32000
      }
    }
  }
}
```

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
- `502`: upstream network/service error (`upstream_unavailable` or `upstream_error`)

Notes:

- The envelope above only applies to errors generated by the gateway.
- Upstream 4xx responses are relayed as-is and may not match the gateway envelope.

## Development

Run tests:

```bash
go test ./...
go test -race ./...
```

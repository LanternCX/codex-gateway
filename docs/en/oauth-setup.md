# OAuth Setup

Language: [English](oauth-setup.md) | [简体中文](../zh-CN/oauth-setup.md)

Docs: [Index](README.md) · [Architecture](architecture.md) · [Privacy Boundary](privacy-boundary.md) · [Git Flow](git-flow.md)

This guide configures `codex-gateway` OAuth login for upstream access.

Default mode is callback-based browser login with local redirect.

## 1) Prepare Runtime Config

Copy sample config and edit values (from repository root):

```bash
cp config.example.yaml config.yaml
```

`--config` must reference a file inside `--workdir`.

Required minimum config for callback mode:

- `auth.downstream_api_key`

`upstream.mode` defaults to `codex_oauth` when omitted.
Set `upstream.base_url` only when using `upstream.mode: openai_api`.

Optional `logging` settings:

- `level` (`debug`, `info`, `warn`, `error`)
- `format` (`text`, `json`)
- `output` (`stdout`, `file`, `both`)
- `color` (`auto`, `always`, `never`, only effective in `text` format)
- `file.dir` (empty means `<workdir>/logs`)
- `file.name`
- `file.max_size_mb`, `file.max_backups`, `file.max_age_days`
- `file.compress`

Optional `oauth` overrides:

- `client_id`
- `authorize_endpoint`
- `device_authorization_endpoint`
- `token_endpoint`
- `redirect_host`
- `redirect_port`
- `redirect_path`
- `scopes`
- `audience`
- `client_secret` (required by some providers)

Optional outbound proxy:

```yaml
network:
  proxy_url: "http://127.0.0.1:7890"
```

- Empty or unset `network.proxy_url` means no explicit proxy.
- `network.proxy_url` must be an absolute URL with host and use one of these schemes: `http`, `https`, `socks5`, `socks5h` (for example, `http://127.0.0.1:7890` or `socks5h://127.0.0.1:1080`).
- When set, this proxy is used for both `auth login` OAuth requests and `serve` upstream forwarding requests.

Note: callback defaults are already set for Codex OAuth, so most users do not need to edit the `oauth` block.

Upstream mode options:

- `codex_oauth` (default): use `https://chatgpt.com/backend-api/codex/responses` compatibility flow
- `openai_api`: direct proxy mode using `upstream.base_url`

## 2) Run Interactive Callback Login (Default)

```bash
./codex-gateway auth login --workdir . --config config.yaml
```

The command will:

1. start a local callback listener (default: `localhost:1455/auth/callback`)
2. print and open browser authorization URL
3. receive callback code and exchange it for OAuth tokens
4. write tokens to `./oauth-token.json`

## 3) Start Gateway Server

```bash
./codex-gateway serve --workdir . --config config.yaml
```

## Troubleshooting

- `oauth token unavailable, run auth login`: token file missing, unreadable, or refresh failed.
- `missing required field`: check `config.yaml` required fields.
- callback login timeout: restart `auth login` and complete browser authorization promptly.

# Privacy Boundary

Language: [English](privacy-boundary.md) | [简体中文](../zh-CN/privacy-boundary.md)

Docs: [Index](README.md) · [Architecture](architecture.md) · [OAuth Setup](oauth-setup.md) · [Git Flow](git-flow.md)

## What Leaves Your Server

When processing `/v1/models` and `/v1/chat/completions`, the gateway forwards request payloads to the configured upstream API (`upstream.base_url`).

Forwarded data can include:

- model id
- chat messages and tool payloads
- generation parameters (`temperature`, `top_p`, etc.)

## What Stays Local

The gateway keeps these local runtime assets in `--workdir`:

- `config.yaml`
- `oauth-token.json`

It does not upload local config files by itself.

## Secrets Handling

- Downstream fixed API key is validated locally.
- OAuth access and refresh tokens are stored locally in `oauth-token.json`.
- Logs should never include downstream API key, access tokens, or refresh tokens.

## Operator Responsibilities

- Restrict filesystem permissions for runtime directory.
- Rotate downstream fixed API key when needed.
- Re-run `auth login` if refresh is revoked or expires.

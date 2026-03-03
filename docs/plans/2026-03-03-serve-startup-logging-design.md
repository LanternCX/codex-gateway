# Serve Startup Logging Design

Date: 2026-03-03
Status: Approved

## Background

The gateway `serve` command currently logs only startup/shutdown lifecycle events.
Operators want immediate runtime hints after startup, including:

- the API prefix clients should use
- currently available models

They also want the default log output to be human-readable with colors in terminals.

## Goals

1. After `serve` starts, print an API prefix suitable for downstream OpenAI-compatible clients.
2. After startup, attempt to discover and log available model IDs.
3. Keep startup resilient: model discovery failures must not block server startup.
4. Switch default log format to terminal-friendly human text with color auto-detection.

## Non-Goals

- No changes to HTTP API behavior.
- No persistent model cache.
- No repeated background model polling.

## Selected Approach (Option B)

Use a one-time, best-effort local probe after `ListenAndServe` starts:

1. Build an API prefix from `server.listen` (for example `http://127.0.0.1:8080/v1`).
2. Log startup fields including `api_prefix`.
3. In a goroutine, call local `GET <api_prefix>/models` with the configured downstream API key.
4. Parse model IDs from the response and log `available_models`.
5. On any probe error, log a warning and continue serving.

This keeps code cohesive in `internal/cli` and avoids hardcoding model sets for `openai_api` mode.

## Implementation Details

### CLI (`internal/cli`)

- Add small helper functions for:
  - API prefix normalization from `cfg.Server.Listen`
  - single-shot local model discovery with short timeout
- Update `runServe` startup flow:
  - include `api_prefix` in startup log
  - kick off non-blocking model discovery and logging

### Logging defaults (`internal/config` + `config.example.yaml`)

- Change default `logging.format` from `json` to `text`.
- Keep `logging.color` default as `auto`.
- Update sample config to match (`format: "text"`).

## Error Handling

- Startup probe errors become warning logs only.
- Invalid or unexpected `/v1/models` response shape becomes warning logs only.
- Existing server lifecycle error handling remains unchanged.

## Test Plan

1. Unit tests for API prefix formatting from listen address variants.
2. Unit tests for model discovery success and graceful failure behavior.
3. Config tests asserting new default `logging.format=text`.
4. Existing package tests for `internal/cli`, `internal/config`, and `internal/logging` remain green.

## Documentation Impact

- Update README (EN/ZH) to state startup logs include API prefix and discovered models.
- Update logging defaults in docs/config example where necessary.

# API Reference

Language: [English](api-reference.md) | [简体中文](../zh-CN/api-reference.md)

Docs: [Index](README.md) · [OpenAPI Spec](../openapi.yaml) · [Architecture](architecture.md)

## Overview

- Base URL (default): `http://127.0.0.1:8080`
- Auth: `Authorization: Bearer <downstream_api_key>` for protected endpoints
- Content type: `application/json` (plus `text/event-stream` for streaming chat)

## Error Envelope

Gateway-generated errors use this schema:

```json
{
  "error": {
    "message": "human readable message",
    "type": "gateway_error",
    "code": "machine_readable_code"
  }
}
```

Common status codes:

- `401`: missing/invalid downstream API key
- `503`: OAuth token unavailable or refresh failed
- `502`: upstream network/service failure

## GET /healthz

Health check endpoint.

- Auth required: No

Response (`200`):

```json
{
  "status": "ok"
}
```

## GET /v1/models

Lists available models.

- Auth required: Yes
- Header: `Authorization: Bearer <downstream_api_key>`

Mode behavior:

- `codex_oauth` (default): returns compatibility model list from gateway
- `openai_api`: proxies upstream `/v1/models` response

Response (`200`, `codex_oauth` example):

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-5.3-codex",
      "object": "model",
      "created": 0,
      "owned_by": "openai"
    },
    {
      "id": "gpt-5.2-codex",
      "object": "model",
      "created": 0,
      "owned_by": "openai"
    }
  ]
}
```

## POST /v1/chat/completions

Creates a chat completion.

- Auth required: Yes
- Header: `Authorization: Bearer <downstream_api_key>`

### Request Body Schema

```json
{
  "model": "string (required)",
  "messages": [
    {
      "role": "system | user | assistant | tool",
      "content": "string | object | array",
      "name": "string (optional)"
    }
  ],
  "stream": false,
  "temperature": 0.7,
  "top_p": 1,
  "max_tokens": 1024,
  "tools": [],
  "tool_choice": "auto"
}
```

Notes:

- `model` and at least one non-system message are required.
- In `codex_oauth` mode, request is translated to Codex backend format.
- In `codex_oauth` mode, `max_tokens` is accepted for compatibility but ignored (not forwarded upstream).
- `tools` and `tool_choice` are accepted in request shape but currently not translated to Codex backend tool execution.

### Non-stream Example

Request payload:

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

Response (`200`):

```json
{
  "id": "resp_07d941e7c010e3290169a52c332e10819188f8e4b992036ed6",
  "object": "chat.completion",
  "created": 1772432435,
  "model": "gpt-5.3-codex",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "hello"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 18,
    "completion_tokens": 5,
    "total_tokens": 23
  }
}
```

### Stream Example

Request payload:

```json
{
  "model": "gpt-5.3-codex",
  "messages": [
    {
      "role": "user",
      "content": "Say hello"
    }
  ],
  "stream": true
}
```

Response (`200`, `Content-Type: text/event-stream`):

```text
data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1772432435,"model":"gpt-5.3-codex","choices":[{"index":0,"delta":{"role":"assistant","content":"he"},"finish_reason":null}]}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1772432435,"model":"gpt-5.3-codex","choices":[{"index":0,"delta":{"content":"llo"},"finish_reason":null}]}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1772432435,"model":"gpt-5.3-codex","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

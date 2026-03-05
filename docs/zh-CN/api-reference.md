# API 参考

语言： [English](../en/api-reference.md) | [简体中文](api-reference.md)

文档导航： [索引](README.md) · [OpenAPI 规范](../openapi.yaml) · [架构说明](architecture.md)

## 概览

- 默认基础地址：`http://127.0.0.1:8080`
- 鉴权：受保护接口需要 `Authorization: Bearer <downstream_api_key>`
- 内容类型：`application/json`（流式 chat/responses 返回 `text/event-stream`）
- 可选请求关联头：`X-Request-ID`（会在响应中回传；缺失时自动生成）

## 错误结构

网关自定义错误返回以下结构：

```json
{
  "error": {
    "message": "human readable message",
    "type": "gateway_error",
    "code": "machine_readable_code"
  }
}
```

常见状态码：

- `401`：下游 API Key 缺失或无效
- `503`：OAuth token 不可用或刷新失败
- `502`：上游网络/服务故障（`error.code` 可能为 `upstream_unavailable` 或 `upstream_error`）

说明：

- 上述 envelope 仅适用于网关自身生成的错误。
- 上游返回的 4xx 会被原样透传，可能不遵循网关错误 envelope。

## GET /healthz

健康检查接口。

- 是否需要鉴权：否

响应（`200`）：

```json
{
  "status": "ok"
}
```

## GET /v1/models

获取模型列表。

- 是否需要鉴权：是
- 请求头：`Authorization: Bearer <downstream_api_key>`

模式行为：

- `codex_oauth`（默认）：返回网关内置兼容模型列表
- `openai_api`：代理上游 `/v1/models`

响应（`200`，`codex_oauth` 部分示例，已截断）：

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

创建聊天补全。

- 是否需要鉴权：是
- 请求头：`Authorization: Bearer <downstream_api_key>`

### 请求体结构

```json
{
  "model": "string (必填)",
  "messages": [
    {
      "role": "system | user | assistant | tool",
      "content": "string | object | array",
      "name": "string (可选)"
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

说明：

- `model` 必填，且至少需要一条非 `system` 消息。
- 在 `codex_oauth` 模式下，请求会转换为 Codex backend responses 格式。
- 在 `codex_oauth` 模式下，`max_tokens`/`max_completion_tokens` 为兼容字段，会被接收但不会向上游透传。
- 在 `codex_oauth` 模式下，`tools`、`tool_choice`、`parallel_tool_calls`、`reasoning_effort`、assistant `tool_calls` 历史以及工具消息（`tool_call_id`）会映射到 Codex backend 的工具语义。
- 当上游产生函数调用时，网关会在非流式与流式响应中返回 chat `tool_calls`，并使用 `finish_reason: "tool_calls"`。

### 非流式示例

请求 payload：

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

响应（`200`）：

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

### 流式示例

请求 payload：

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

响应（`200`，`Content-Type: text/event-stream`）：

```text
data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1772432435,"model":"gpt-5.3-codex","choices":[{"index":0,"delta":{"role":"assistant","content":"he"},"finish_reason":null}]}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1772432435,"model":"gpt-5.3-codex","choices":[{"index":0,"delta":{"content":"llo"},"finish_reason":null}]}

data: {"id":"chatcmpl-...","object":"chat.completion.chunk","created":1772432435,"model":"gpt-5.3-codex","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

## POST /v1/responses

通过 responses API 透传创建响应。

- 是否需要鉴权：是
- 请求头：`Authorization: Bearer <downstream_api_key>`（网关配置中的固定 key）

模式行为：

- `codex_oauth`（默认）：代理到 Codex backend responses 路径（默认 `/backend-api/codex/responses`，可通过 `upstream.codex_responses_path` 配置）
- `openai_api`：代理到上游 responses 路径（默认 `/v1/responses`，也可配置）

请求体：

- 网关会先校验非空请求体是否为 JSON；无效 JSON 返回 `400 invalid_request`。
- 在 `codex_oauth` 模式下，如果 `instructions` 缺失或为空，网关会先自动补默认值（`"You are a helpful assistant."`）。
- 在 `codex_oauth` 模式下，`max_output_tokens`/`max_completion_tokens` 为兼容字段，会被接收但在转发前移除。
- 完成校验/标准化后，网关会将请求体转发到上游 responses 接口。

响应（`200`）：

- 非流式请求返回 JSON（`application/json`）。
- 流式请求返回 SSE（`text/event-stream`），并透传上游分片。

JSON 响应示例：

```json
{
  "id": "resp_123",
  "object": "response"
}
```

SSE 响应示例：

```text
data: {"id":"resp_1","object":"response.chunk"}

data: [DONE]
```

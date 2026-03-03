# codex-gateway

语言： [English](README.md) | [简体中文](README.zh-CN.md)

这是一个自托管网关，用于：

- 接收 OpenAI 兼容的下游请求（`/v1/models`、`/v1/chat/completions`、`/v1/responses`）
- 通过交互式 CLI OAuth 登录获取上游访问令牌
- 使用配置中的固定 API Key 保护下游访问

## 功能

- 默认交互式 OAuth Callback 登录：`codex-gateway auth login`
- 运行目录存储（`config.yaml`、`oauth-token.json`）
- 默认上游模式是 `codex_oauth`（兼容 ChatGPT OAuth token）
- OpenAI 兼容接口：
  - `GET /v1/models`
  - `POST /v1/chat/completions`（支持流式返回）
  - `POST /v1/responses`（支持 JSON 与流式透传）
  - 在 `codex_oauth` 模式下，`/v1/models` 返回兼容模型列表；`/v1/chat/completions` 会转换为 Codex responses 后端请求并将结果映射回 OpenAI chat 格式；`/v1/responses` 会代理到 Codex responses 后端路径（默认 `/backend-api/codex/responses`，可通过 `upstream.codex_responses_path` 配置）。
  - 在 `openai_api` 模式下，`/v1/chat/completions` 与 `/v1/responses` 会代理到上游路径。
- 固定下游 API Key 鉴权：`Authorization: Bearer <fixed_key>`
- 上游 OAuth 令牌自动刷新
- 结构化日志（可配置 level/format/output/color 与文件滚动策略）
- 默认 stdout 日志为人类可读文本格式，并自动检测终端颜色输出
- 请求关联追踪：支持 `X-Request-ID`（缺失时自动生成）
- 健康检查接口：`GET /healthz`

## 文档

- 文档索引： [docs/zh-CN/README.md](docs/zh-CN/README.md)
- 架构说明： [docs/zh-CN/architecture.md](docs/zh-CN/architecture.md)
- API 参考： [docs/zh-CN/api-reference.md](docs/zh-CN/api-reference.md)
- OpenAPI 规范： [docs/openapi.yaml](docs/openapi.yaml)
- OAuth 配置： [docs/zh-CN/oauth-setup.md](docs/zh-CN/oauth-setup.md)
- 隐私边界： [docs/zh-CN/privacy-boundary.md](docs/zh-CN/privacy-boundary.md)
- Git 工作流： [docs/zh-CN/git-flow.md](docs/zh-CN/git-flow.md)
- English docs： [docs/en/README.md](docs/en/README.md)

## 运行目录

默认情况下，运行文件基于 `--workdir`（默认为当前目录）解析：

- `config.yaml`
- `oauth-token.json`
- 结构化日志可输出到 stdout 或文件（`logging.output`）
- 当 `logging.output` 为 `file` 或 `both` 时，会使用 `logs/`（默认 `<workdir>/logs`）

运行路径策略：

- `--config` 必须指向 `--workdir` 内部的文件
- 网关生成的运行时文件仅写入 `--workdir`

上游模式：

- `upstream.mode: codex_oauth`（默认）：将 chat-completions 转换后发送到 Codex backend responses
- `upstream.mode: openai_api`：直接代理到 `upstream.base_url`

## 快速开始

1）构建：

```bash
go build -o codex-gateway ./cmd/codex-gateway
```

2）准备配置（在仓库根目录执行）：

```bash
cp config.example.yaml config.yaml
```

然后编辑 `config.yaml`，至少填写 `auth.downstream_api_key`。
未设置时 `upstream.mode` 默认为 `codex_oauth`；仅在 `upstream.mode: openai_api` 时需要设置 `upstream.base_url`。
对于 Codex OAuth 的 callback 模式，OAuth 端点和 `client_id` 已有默认值。
如有需要，可为 `auth login` 与 `serve` 的外发请求配置统一代理：

```yaml
network:
  proxy_url: "http://127.0.0.1:7890"
```

`network.proxy_url` 必须是包含 host 的绝对 URL，且协议仅支持 `http`、`https`、`socks5`、`socks5h`（例如 `http://127.0.0.1:7890` 或 `socks5h://127.0.0.1:1080`）。
将 `network.proxy_url` 留空或不设置时，表示不显式配置代理。

3）执行 OAuth 登录（交互式）：

```bash
./codex-gateway auth login --workdir . --config config.yaml
```

该命令会启动本地 callback 监听并尝试自动打开浏览器授权页面。

4）启动服务：

```bash
./codex-gateway serve --workdir . --config config.yaml
```

启动后日志会包含：

- `api_prefix`（例如 `http://127.0.0.1:8080/v1`）
- 通过启动探测（`GET /v1/models`）发现的 `available_models`

## API 参考

- 正式 API 文档： [docs/zh-CN/api-reference.md](docs/zh-CN/api-reference.md)
- 可导入 Apifox 的 OpenAPI 文件： [docs/openapi.yaml](docs/openapi.yaml)

接口摘要：

- `GET /healthz`
- `GET /v1/models`
- `POST /v1/chat/completions`
- `POST /v1/responses`

## OpenCode 自定义 Provider

当 OpenCode 客户端连接本网关并希望获得类似 codex 的 responses/thinking 行为时，建议自定义 provider 使用 `@ai-sdk/openai`（而不是泛 OpenAI-compatible 适配器）。

`opencode.json` 示例：

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

请求 payload 示例（`POST /v1/chat/completions`）：

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

## 错误码

网关错误返回 OpenAI 风格 envelope：

```json
{
  "error": {
    "message": "...",
    "type": "gateway_error",
    "code": "..."
  }
}
```

常见状态码：

- `401`：下游固定 API Key 缺失或错误
- `503`：OAuth 令牌不可用或刷新失败
- `502`：上游网络/服务错误（`upstream_unavailable` 或 `upstream_error`）

说明：

- 上述 envelope 仅适用于网关自身生成的错误。
- 上游 4xx 响应会原样透传，可能不符合网关 envelope。

## 开发

运行测试：

```bash
go test ./...
go test -race ./...
```

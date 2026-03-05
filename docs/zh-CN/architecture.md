# codex-gateway 架构说明

语言： [English](../en/architecture.md) | [简体中文](architecture.md)

文档导航： [索引](README.md) · [API 参考](api-reference.md) · [OpenAPI 规范](../openapi.yaml) · [OAuth 配置](oauth-setup.md) · [隐私边界](privacy-boundary.md) · [Git 工作流](git-flow.md)

## 概览

`codex-gateway` 是一个单二进制 Go 服务，提供：

- 通过 CLI 执行交互式 OAuth 登录（`auth login`）
  - callback 模式（默认，本地浏览器回调）
- OpenAI 兼容 HTTP 接口（`/v1/models`、`/v1/chat/completions`）
- 固定下游 API Key 鉴权

网关通过运行目录中的 OAuth 令牌文件完成上游认证。

## 运行目录布局

默认情况下，运行文件基于当前目录（或 `--workdir`）解析：

- `config.yaml`：服务、鉴权、OAuth、上游配置
- `oauth-token.json`：持久化的 access/refresh token
- 结构化日志：由 `logging.output` 控制输出到 `stdout`、`file` 或 `both`；文本颜色由 `logging.color` 控制
- 启用文件日志时，默认目录为 `<workdir>/logs`

## 包边界

- `cmd/codex-gateway`：程序入口
- `internal/cli`：Cobra 命令装配（`serve`、`auth login`）
- `internal/config`：YAML 配置加载与校验
- `internal/logging`：日志构建、敏感字段脱敏、请求 ID 透传辅助与多输出处理
- `internal/auth`：令牌持久化与刷新策略管理
- `internal/oauth`：OAuth Device Flow 与 refresh 请求
- `internal/upstream`：上游 HTTP 客户端封装
- `internal/server`：下游 HTTP 服务、鉴权中间件、错误映射与代理处理

## 请求流程

1. 下游客户端携带 `Authorization: Bearer <fixed-key>` 调用 `/v1/models` 或 `/v1/chat/completions`。
2. `internal/server` 校验固定下游 API Key。
3. Handler 向 token manager 获取有效上游 access token。
4. 如 token 临近过期，manager 调用 OAuth token endpoint 刷新并改写 token 文件。
5. 网关转发请求到上游 OpenAI 兼容端点，并回传响应。
6. 在 `codex_oauth` 模式下，网关会将 chat-completions 转换为 Codex responses 后端格式，再转换回 OpenAI chat-completions 响应。
7. 在 `openai_api` 模式下，网关直接代理上游请求和响应。

`/v1/models` 行为：

- `codex_oauth` 模式：返回静态兼容模型列表（Codex 可用模型 ID）。
- `openai_api` 模式：代理上游 `/v1/models`。

## 错误映射

- `401`：下游固定 API Key 缺失或错误
- `503`：OAuth token 缺失或刷新失败
- `502`：上游请求失败或上游 5xx 映射失败

响应使用 OpenAI 风格错误结构：

```json
{
  "error": {
    "message": "...",
    "type": "gateway_error",
    "code": "..."
  }
}
```

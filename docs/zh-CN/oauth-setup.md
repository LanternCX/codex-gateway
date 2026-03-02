# OAuth 配置

语言： [English](../en/oauth-setup.md) | [简体中文](oauth-setup.md)

文档导航： [索引](README.md) · [架构说明](architecture.md) · [隐私边界](privacy-boundary.md) · [Git 工作流](git-flow.md)

本文用于配置 `codex-gateway` 的 OAuth 登录流程，以便访问上游服务。

默认模式是 callback 浏览器登录（本地回调）。

## 1）准备运行配置

先复制示例配置并编辑：

```bash
cp config.example.yaml config.yaml
```

`--config` 必须指向 `--workdir` 内的文件。

callback 模式下最小必填配置：

- `auth.downstream_api_key`
- `upstream.mode`（默认 `codex_oauth`）

可选 `logging` 配置：

- `level`（`debug`、`info`、`warn`、`error`）
- `format`（`text`、`json`）

`oauth` 可选覆盖字段：

- `client_id`
- `authorize_endpoint`
- `token_endpoint`
- `redirect_host`
- `redirect_port`
- `redirect_path`
- `scopes`
- `audience`
- `client_secret`（部分 OAuth 提供方要求）

说明：Codex OAuth 的 callback 默认值已内置，大多数场景无需修改 `oauth` 配置块。

上游模式：

- `codex_oauth`（默认）：使用 `https://chatgpt.com/backend-api/codex/responses` 兼容流程
- `openai_api`：按 `upstream.base_url` 进行直接代理

## 2）执行交互式 Callback 登录（默认）

```bash
./codex-gateway auth login --workdir . --config config.yaml
```

该命令会：

1. 启动本地回调监听（默认 `localhost:1455/auth/callback`）
2. 输出并尝试打开浏览器授权 URL
3. 接收 callback code 并换取 OAuth token
4. 将 token 写入 `./oauth-token.json`

## 3）启动网关服务

```bash
./codex-gateway serve --workdir . --config config.yaml
```

## 故障排查

- `oauth token unavailable, run auth login`：token 文件缺失、不可读或 refresh 失败。
- `missing required field`：检查 `config.yaml` 的必填字段。
- callback 登录超时：重新执行 `auth login`，并尽快完成浏览器授权。

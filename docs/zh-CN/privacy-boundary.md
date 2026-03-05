# 隐私边界

语言： [English](../en/privacy-boundary.md) | [简体中文](privacy-boundary.md)

文档导航： [索引](README.md) · [架构说明](architecture.md) · [OAuth 配置](oauth-setup.md) · [Git 工作流](git-flow.md)

## 会离开服务器的数据

处理 `/v1/models` 与 `/v1/chat/completions` 时，网关会把请求负载转发到配置的上游 API（`upstream.base_url`）。

可能转发的数据包括：

- 模型 ID
- 聊天消息与工具调用负载
- 生成参数（`temperature`、`top_p` 等）

## 保留在本地的数据

网关会在 `--workdir` 目录保留以下运行文件：

- `config.yaml`
- `oauth-token.json`

网关不会主动上传本地配置文件。

## 密钥处理

- 下游固定 API Key 仅在本地校验。
- OAuth access token 与 refresh token 保存在本地 `oauth-token.json`。
- 日志系统会对已知敏感字段做脱敏（例如 `authorization`、`api_key`、`access_token`、`refresh_token`、`client_secret`），避免密钥泄露。

## 运维责任

- 限制运行目录文件权限。
- 按需轮换固定下游 API Key。
- refresh 被撤销或失效时重新执行 `auth login`。

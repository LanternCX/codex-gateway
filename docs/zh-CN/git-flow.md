# Git 工作流

语言： [English](../en/git-flow.md) | [简体中文](git-flow.md)

文档导航： [索引](README.md) · [架构说明](architecture.md) · [OAuth 配置](oauth-setup.md) · [隐私边界](privacy-boundary.md)

## 分支模型

- 长期分支：`main`、`dev`
- 功能分支：`feat/*` 或 `feature/*`（从 `dev` 拉出）

## 日常流程

1. 从 `dev` 创建功能分支。
2. 以小而可审查的提交逐步实现。
3. 先跑定向测试，再跑全量测试：

```bash
go test ./...
go test -race ./...
```

4. 向 `dev` 发起 PR。
5. 验证通过后，再将 `dev` 合并到 `main` 发布。

## 提交规范

使用 Angular 风格 Conventional Commits：

- `feat:` 新功能
- `fix:` 修复问题
- `docs:` 文档变更
- `chore:` 工具链或维护变更

示例：

- `feat: add oauth device login command`
- `fix: map upstream 5xx to gateway 502`
- `docs: add oauth setup guide`

## 发布流程

- 在 `main` 打 `v*` 标签（例如 `v0.1.0`）。
- 推送标签会触发 [`.github/workflows/package.yml`](../../.github/workflows/package.yml)，先执行 `go test ./...` 和 `go test -race ./...`，再进行打包。
- 发布打包目标平台：`macos-arm`、`windows-x64`、`linux-x64`、`linux-arm`。
- 每个发布包包含：可执行文件、`README.md`、`README.zh-CN.md`、`config.example.yaml`。

# codex-gateway Agent Guide

## Purpose
This file defines repository-specific operating rules for coding agents working in codex-gateway.
Prefer existing project patterns over invention.

## Project Snapshot
- Language: Go (`1.26+`)
- CLI framework: `cobra`
- HTTP stack: `net/http`
- Config format: YAML (`gopkg.in/yaml.v3`)
- Testing: `go test` + `httptest`

## Sources Of Truth
- [README.md](README.md)
- [README.zh-CN.md](README.zh-CN.md)
- [docs/en/README.md](docs/en/README.md)
- [docs/zh-CN/README.md](docs/zh-CN/README.md)
- [docs/en/architecture.md](docs/en/architecture.md)
- [docs/zh-CN/architecture.md](docs/zh-CN/architecture.md)
- [docs/en/oauth-setup.md](docs/en/oauth-setup.md)
- [docs/zh-CN/oauth-setup.md](docs/zh-CN/oauth-setup.md)
- [docs/en/privacy-boundary.md](docs/en/privacy-boundary.md)
- [docs/zh-CN/privacy-boundary.md](docs/zh-CN/privacy-boundary.md)
- [docs/en/git-flow.md](docs/en/git-flow.md)
- [docs/zh-CN/git-flow.md](docs/zh-CN/git-flow.md)
- [.opencode/skills/code-standard/SKILL.md](.opencode/skills/code-standard/SKILL.md)
- [.opencode/skills/doc-maintainer/SKILL.md](.opencode/skills/doc-maintainer/SKILL.md)
- [.opencode/skills/git-workflow/SKILL.md](.opencode/skills/git-workflow/SKILL.md)

## Environment Setup

```bash
go mod download
```

## Build, Lint, Test Commands

### Core Verification

```bash
go test ./...
go test -race ./...
go build ./cmd/codex-gateway
```

### Focused Test Workflows

```bash
go test ./internal/server -run TestProxy -v
go test ./internal/oauth -run TestClient_AuthenticateSuccess -v
go test ./internal/auth -run TestManager -v
```

## Architecture Boundaries
Maintain one-way dependency flow:

`config/oauth/auth -> upstream -> server -> cli`

Rules:
- Keep transport logic in `internal/server` and `internal/upstream`.
- Keep token lifecycle logic in `internal/auth` and OAuth protocol logic in `internal/oauth`.
- Keep `internal/cli` thin (wiring and process orchestration only).
- Avoid hidden globals; pass dependencies explicitly.

## Code Style Guidelines

### Imports and Packages
- Use module-rooted imports (`codex-gateway/internal/...`).
- Keep package responsibilities focused.
- Avoid import-time side effects.

### Formatting
- Use `gofmt` for all Go source files.
- Add comments only when logic is non-obvious.
- Keep production code and comments in English.

### Types and Error Handling
- Prefer explicit struct contracts at package boundaries.
- Return actionable errors with context (`fmt.Errorf("...: %w", err)`).
- Never silently swallow errors.

## Testing Expectations
- Prefer TDD for feature and bugfix work.
- Keep tests near package behavior (`*_test.go` in same package directory).
- Cover happy paths and edge/failure paths.
- Run targeted tests first, then full suite for substantial changes.

## Documentation Expectations
- If behavior changes, update [README.md](README.md), [README.zh-CN.md](README.zh-CN.md), and relevant docs under [docs/](docs/en/README.md).
- Keep architecture and privacy claims aligned with implementation.
- Do not document features as complete before tests pass.

## Git Workflow Expectations
- Branches: `main`, `dev`, and `feat/*` or `feature/*`.
- Commit style: Angular conventional commits (`feat:`, `fix:`, `docs:`, `chore:`).
- Merge feature branches into `dev` first; merge `dev` to `main` for release.

## Agent Checklist Before Finishing
1. Changes follow architecture and style constraints.
2. Relevant targeted tests were run.
3. Full test suite was run for substantial changes.
4. User-facing docs were updated for behavior changes.
5. No conflict with [.opencode/skills/](.opencode/skills/) rules.

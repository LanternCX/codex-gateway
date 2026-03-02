# Git Flow

Language: [English](git-flow.md) | [简体中文](../zh-CN/git-flow.md)

Docs: [Index](README.md) · [Architecture](architecture.md) · [OAuth Setup](oauth-setup.md) · [Privacy Boundary](privacy-boundary.md)

## Branch Model

- Long-lived branches: `main`, `dev`
- Feature branches: `feat/*` or `feature/*` (branch from `dev`)

## Daily Workflow

1. Create a feature branch from `dev`.
2. Implement changes in small, reviewable commits.
3. Run targeted tests first, then full suite:

```bash
go test ./...
go test -race ./...
```

4. Open PR into `dev`.
5. After validation, merge `dev` into `main` for release.

## Commit Convention

Use Angular-style conventional commits:

- `feat:` new user-visible capability
- `fix:` behavior correction
- `docs:` documentation only
- `chore:` tooling or maintenance

Examples:

- `feat: add oauth device login command`
- `fix: map upstream 5xx to gateway 502`
- `docs: add oauth setup guide`

## Release Workflow

- Tag releases from `main` as `v*` (for example `v0.1.0`).
- Tag push triggers [.github/workflows/package.yml](../.github/workflows/package.yml) to build cross-platform artifacts and draft a GitHub release.

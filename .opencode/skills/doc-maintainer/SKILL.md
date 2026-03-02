---
name: doc-maintainer
description: Use when updating repository documentation so README, architecture notes, privacy boundary, OAuth setup, and git-flow guidance stay consistent.
---

# Doc Maintainer

## Overview
This skill keeps codex-gateway documentation complete and synchronized.
Core principle: every meaningful behavior change must be reflected in user-facing and maintainer-facing docs.

## Required Documents
- [README.md](../../../README.md) for English quickstart and API usage.
- [README.zh-CN.md](../../../README.zh-CN.md) for Chinese quickstart and API usage.
- [docs/en/architecture.md](../../../docs/en/architecture.md) and [docs/zh-CN/architecture.md](../../../docs/zh-CN/architecture.md) for package boundaries and data flow.
- [docs/en/oauth-setup.md](../../../docs/en/oauth-setup.md) and [docs/zh-CN/oauth-setup.md](../../../docs/zh-CN/oauth-setup.md) for OAuth configuration and login workflow.
- [docs/en/privacy-boundary.md](../../../docs/en/privacy-boundary.md) and [docs/zh-CN/privacy-boundary.md](../../../docs/zh-CN/privacy-boundary.md) for data-sharing and secret-handling contract.
- [docs/en/git-flow.md](../../../docs/en/git-flow.md) and [docs/zh-CN/git-flow.md](../../../docs/zh-CN/git-flow.md) for branch and release workflow.

## Update Rules
- Keep English and Chinese docs aligned in the same change.
- Keep CLI flags and command examples runnable.
- Keep privacy claims strict and verifiable in code paths.
- Do not document features as complete before tests pass.
- Keep onboarding order practical: configure -> login -> serve -> verify.
- Use markdown hyperlinks when referencing internal documentation paths.

## Release Note Rules
- Explain why the change exists, not only what changed.
- Link changed files and test evidence in PR description.

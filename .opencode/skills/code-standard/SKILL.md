---
name: code-standard
description: Use when adding or modifying Go source code in this repository and architecture boundaries, conventions, or test expectations need to be enforced.
---

# Code Standard

## Overview
This skill defines coding standards for codex-gateway implementation work.
Core principle: keep packages cohesive, interfaces explicit, and side effects isolated.

## Rules
- Write all production code and comments in English.
- Prefer small packages with clear responsibilities.
- Keep dependency flow one-way: `config/oauth/auth -> upstream -> server -> cli`.
- Avoid hidden globals; pass dependencies explicitly.
- Return actionable wrapped errors (`fmt.Errorf("...: %w", err)`).
- Keep networking and process side effects at boundaries (`internal/server`, `internal/upstream`, `internal/cli`).

## Testing Contract
- Follow TDD for new behavior and bug fixes.
- Add behavior-focused tests in matching package directories.
- Cover edge and failure paths for auth and proxy behavior.

## Quick Review Checklist
- Does this change increase coupling across layers?
- Are package boundaries still clear?
- Are failure paths and regressions tested?
- Is naming explicit and consistent?

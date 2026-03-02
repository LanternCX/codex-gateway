---
name: git-workflow
description: Use when performing branch, merge, and commit operations in this repository under Git Flow and Angular conventional commit constraints.
---

# Git Workflow

## Overview
This skill enforces codex-gateway repository git workflow requirements.
Core principle: integrate through `dev`, release through `main`, and keep history auditable.

## Branch Model
- Long-lived branches: `main`, `dev`.
- Feature branches: `feat/*` or `feature/*`, branched from `dev`.

## Commit Convention
- Use Angular conventional commit style: `feat:`, `fix:`, `docs:`, `chore:`.
- Keep commit scope small and reviewable.

## Merge Policy
- Run tests before merge.
- Merge feature branches into `dev` first.
- Merge `dev` into `main` only after verification gates pass.

## Safety Rules
- Do not use destructive git commands without explicit approval.
- Do not force-push protected branches.
- Keep release tags in `v*` format for automation.

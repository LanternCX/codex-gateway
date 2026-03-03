# codex-gateway Progress Log

## Entry Template

```markdown
# YYYY-MM-DD-N

## Date
YYYY-MM-DD

## Title
[Short actionable title]

## Background / Issue
[Context, trigger, constraints]

## Actions / Outcome
- Approach 1: [what was tried] -> [result]
- Approach 2: [what was tried] -> [result]
- Final approach: [adopted approach] -> [why it worked]

## Lessons / Refinements
- [Reusable pattern]
- [Avoidance note]

## Related Commit Message
type(scope): summary

## Related Commit Hash
abc123d (or TBD)
```

## Global TOC

| Page ID | Date | Title | Path | Keywords |
| --- | --- | --- | --- | --- |
| 2026-03-02-1 | 2026-03-02 | Implement Codex OAuth OpenAI-compatible gateway MVP | `.progress/entries/2026/2026-03-02-1.md` | oauth, openai-compatible, gateway, go |
| 2026-03-02-2 | 2026-03-02 | Initialize repository workflow, governance docs, and OpenCode scaffolding | `.progress/entries/2026/2026-03-02-2.md` | git-flow, docs, opencode, progress |
| 2026-03-02-3 | 2026-03-02 | Add bilingual documentation and hyperlink-first navigation | `.progress/entries/2026/2026-03-02-3.md` | docs, i18n, hyperlinks, navigation |
| 2026-03-02-4 | 2026-03-02 | Switch OAuth login default to callback mode with minimal config defaults | `.progress/entries/2026/2026-03-02-4.md` | oauth, callback, pkce, config, docs |
| 2026-03-02-5 | 2026-03-02 | Debug callback unknown_error and align OAuth authorize request with opencode | `.progress/entries/2026/2026-03-02-5.md` | oauth, callback, debug, compatibility |
| 2026-03-02-6 | 2026-03-02 | Enforce runtime-directory-only config and token path policy | `.progress/entries/2026/2026-03-02-6.md` | runtime, workdir, path-guard, docs |
| 2026-03-02-7 | 2026-03-02 | Align upstream flow with opencode Codex OAuth compatibility | `.progress/entries/2026/2026-03-02-7.md` | oauth, codex, proxy, compatibility |
| 2026-03-02-8 | 2026-03-02 | Add formal bilingual API documentation and OpenAPI spec | `.progress/entries/2026/2026-03-02-8.md` | docs, api, openapi, i18n |
| 2026-03-02-9 | 2026-03-02 | Fix opencode compatibility by removing unsupported max_output_tokens forwarding | `.progress/entries/2026/2026-03-02-9.md` | opencode, compatibility, chat, payload |
| 2026-03-02-10 | 2026-03-02 | Finalize callback-only mainline and split device flow into hold branch | `.progress/entries/2026/2026-03-02-10.md` | git-flow, callback, device-branch, docs |
| 2026-03-02-11 | 2026-03-02 | Upgrade gateway logging to industrial-grade multi-sink structured telemetry | `.progress/entries/2026/2026-03-02-11.md` | logging, observability, request-id, redaction |
| 2026-03-03-1 | 2026-03-03 | Log API prefix and discovered models on serve startup | `.progress/entries/2026/2026-03-03-1.md` | serve, startup, logging, models, ux |

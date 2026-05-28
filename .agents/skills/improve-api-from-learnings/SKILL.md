---
name: improve-api-from-learnings
description: Use when improving Nevermind's extension API or builder workflow from accumulated learning traces. Export learnings first, then analyze the raw artifacts to propose API, prompt, or builder changes.
---

# Improve API From Learnings

Use this skill when the goal is to improve Nevermind itself from past extension-building friction, not when building a user extension.

Workflow:

1. Run `mise exec pnpm -- pnpm learnings:export`.
2. Read the newest export bundle under `.tmp/learnings-export/`.
3. Treat `traces.json` as the main evidence stream.
4. Treat `learnings.md` and `learnings.json` as the current user-learning output, not the source of truth for product changes.
5. Identify repeated friction in how extensions are built: missing API primitives, confusing names, prompt failures, weak tool descriptions, repeated retries, or host gaps.
6. Prefer improving `src/resources/nevermind-extension-api.d.ts`, `src/electron/ai.ts`, the builder skill, or host behavior at the source instead of adding workaround instructions.
7. Make small reviewable changes and verify with `mise exec pnpm -- pnpm test`.

Rules:

- Do not use this skill for user-extension authoring; use the normal extension-builder flow for that.
- Do not treat one-off chat details as product learnings; look for repeated or clearly generalizable friction.
- Do not bloat prompts with many new rules; prefer fixing API shape, tool descriptions, or host primitives.
- When prompt changes are needed, tighten or replace existing guidance instead of appending parallel instructions.
- When API changes are needed, update `src/resources/nevermind-extension-api.d.ts` as the canonical contract.
- Keep developer learnings grounded in raw exported traces, especially tool calls, validation cycles, and runtime failures.

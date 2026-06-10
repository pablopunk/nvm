---
name: improve-api-from-conversations
description: Use when improving Nevermind's extension API or builder workflow from recent local AI builder conversations. Review persisted aiChats and generated extension code, then draft actionable GitHub issues for API, host, or builder improvements.
---

# Improve API From Conversations

Use this skill when the goal is to improve Nevermind itself from recent extension-building conversations, not when building a user extension.

This skill intentionally uses local AI builder conversations as the primary evidence stream. Automatic learning exports can be useful background, but they are not the source of truth for product/API changes.

Workflow:

1. Explore the repo first: inspect `git status --short`, recent commits, current diffs, and relevant source before drawing conclusions.
2. Read recent local AI builder chats from Nevermind's persisted state as read-only evidence. Do not edit app-owned state files.
3. Identify chats with product/API friction: repeated retries, user complaints, generated extension runtime failures, confusing API usage, duplicate extensions, missing host primitives, blocked UI, broken loading states, or builder prompt failures.
4. Inspect the generated extension files referenced by those chats from the installed extensions directory. Compare the conversation symptoms with the actual extension code and host API behavior.
5. Check whether recent commits, current diffs, or source changes already address the root cause. Mark those findings as already addressed instead of rediscovering them as new work.
6. Group remaining evidence by root cause and classify each group as an extension API gap, host behavior gap, builder/tooling prompt gap, documentation/type-contract gap, or extension-authoring anti-pattern caused by missing primitives.
7. Prefer fixing API shape, host behavior, or typed contract guidance at the source instead of adding workaround instructions.
8. Draft one or more GitHub issue proposals. Split issues by independently shippable root cause; combine tightly coupled symptoms when one API change should solve them together.
9. Stop before creating issues unless the user explicitly asks. Provide copy-paste-ready issue bodies or `gh issue create` commands when helpful.

Issue proposal format:

- Title
- Problem / symptoms
- User-visible impact
- Evidence from conversations and generated extension code, summarized without raw private chat dumps
- Likely root cause in the extension API, host, or builder workflow
- Proposed API/host/builder changes
- Migration and backward-compatibility notes
- Acceptance criteria
- Suggested labels and priority
- Already-addressed related fixes, if any

Rules:

- Do not use this skill for user-extension authoring; use the normal extension-builder flow for that.
- Do not persist raw conversation contents, secrets, personal data, or local-only paths in issue bodies unless the path names stable product files or extension API surfaces.
- Treat local chats and installed generated extensions as private evidence. Summarize product/API friction rather than quoting long user conversations.
- Do not edit app-owned state while Nevermind is running; this workflow should only read state.
- Do not treat one-off chat details as product issues unless the user explicitly identifies them as broadly important or the API shape made the failure likely.
- When prompt changes are needed, tighten or replace existing guidance instead of appending parallel instructions.
- When API changes are needed, update `src/resources/nevermind-extension-api.d.ts` as the canonical contract in the implementation ticket.
- Before proposing a new issue, search current source, recent commits, and open diffs for the same root cause so already-shipped fixes are credited rather than duplicated.

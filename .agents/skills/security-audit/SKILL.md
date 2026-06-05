---
name: security-audit
description: Use when reviewing Nevermind for security risk, threat modeling, secure-code review, dependency/config exposure, secrets, OWASP-style web/API issues, Electron attack surface, auth/session/token/billing abuse, or any request for a penetration-test/security report. Explore the repo before conclusions; this is for authorized review of this app only.
---

# Security Audit

Use this skill for defensive security work on Nevermind. Produce evidence-backed findings, not generic advice.

## First steps

1. **Establish scope from the repo.** Identify the desktop app, backend, deployed surfaces, auth providers, data stores, IPC boundaries, extension APIs, and privileged OS actions before judging risk.
2. **Check local guidance.** Read `AGENTS.md`, then the relevant project skill files. For Electron-specific work, read `../electron-best-practices/SKILL.md` and linked files.
3. **Map trust boundaries.** Renderer ↔ preload ↔ main, extensions ↔ host API, user content ↔ extension views, desktop ↔ backend, backend ↔ upstream AI providers, auth sessions ↔ API tokens, admin ↔ user.
4. **Audit high-impact paths first.** Auth, tokens, admin APIs, proxy/billing, custom protocols, shell/file/URL actions, IPC handlers, extension permissions, secrets/config, update/release pipeline.
5. **Verify findings.** Prefer code references, concrete reproduction steps, safe local tests, and exact commands over speculation.

## Nevermind security surfaces

- Electron main/preload/renderer: `src/electron/main.ts`, `src/electron/preload.ts`, `src/electron/palette-window.ts`, `src/preload-api.ts`.
- Extension host and API: `src/resources/nevermind-extension-api.d.ts`, `src/electron/main.ts`, `src/extension-view.tsx`, internal extensions under `src/resources/`.
- Desktop auth/model proxy integration: `src/electron/nevermind-auth.ts`, `src/electron/ai.ts`.
- Astro backend routes: `backend/src/pages/api/**`, especially auth, device auth, admin, tokens, and `/api/v1/**` proxy routes.
- Backend libraries: `backend/src/lib/workos.ts`, `tokens.ts`, `admin.ts`, `proxy.ts`, `ratelimit.ts`, `cron-auth.ts`, `audit.ts`, `settings.ts`.
- Persistence/config: `backend/src/db/schema.ts`, Drizzle migrations, `electron-builder.yml`, `backend/vercel.json`, env examples.

## Audit checklist

- Auth/session: redirect handling, sealed cookie settings, CSRF on cookie-authenticated mutations, logout, device-code approval/exchange, role escalation.
- API tokens: generation entropy, prefix leakage, hashing, revocation, last-use writes, bearer extraction, rate limits, token exposure in logs/UI.
- Authorization: admin gates, object ownership checks, extension permission checks, IPC caller assumptions, path/URL action ownership.
- Electron hardening: context isolation, sandbox, node integration, navigation/window-open policy, permission handlers, CSP/custom protocol behavior, preload API input validation.
- Extension isolation: least-privilege permissions, shell/system actions, file and app helpers, iframe sandbox, untrusted HTML/media, clipboard and AI permissions.
- Backend proxy/billing: transparent auth swap, forwarded headers, request size/token limits, upstream URL construction, streaming parsing, abuse/rate limits, billing failure modes.
- Input/output safety: SSR/script injection, DOM insertion, markdown/rendered content, URL/path validation, shell argument handling, log redaction.
- Secrets/config: committed env files, Vercel env exposure, Sentry DSNs, WorkOS/Redis/Neon keys, release signing material, local auth storage.
- Supply chain: dependency vulnerabilities, Electron version posture, packaged resources, updater/release artifact trust, native modules.

## Report format

For a full report, include:

1. Scope and methodology.
2. Architecture/trust-boundary summary.
3. Executive summary with severity counts.
4. Findings sorted by severity, each with: title, severity, affected files/routes, evidence, impact, reproduction or exploit sketch, recommended fix, verification command/test.
5. Positive controls already present.
6. Residual risks and follow-up hardening roadmap.

Do not print secrets, live tokens, cookies, raw user data, or production payloads. Redact values while preserving enough shape to prove the issue.

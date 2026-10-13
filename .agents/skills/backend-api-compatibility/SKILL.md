---
name: backend-api-compatibility
description: Use when changing or reviewing Nevermind desktop/backend contracts - Astro API routes used by desktop, Electron backend fetches, auth/device login, token revoke, active-model descriptors, AI proxy routes, billing/rate-limit/error shapes, compatibility manifests, backend deploy policy, desktop release/update interactions, or any request about keeping frontend and backend in sync.
---

# Backend API Compatibility

Nevermind's Electron desktop app ships on tagged releases while the backend may deploy continuously. Treat the backend as a backwards-compatible service for installed desktop clients.

## Start here

1. Read `docs/backend-api-compatibility.md`.
2. Map both sides of the contract, including supported release tags rather than only current `main`, before changing code:
   - Desktop callers in `src/app/electron/nevermind-auth.ts`, `src/app/electron/ai.ts`, and related main-process flows.
   - Backend routes in `src/backend/src/pages/api/**` and shared backend libraries in `src/backend/src/lib/**`.
3. Identify the change type: additive, compatibility shim, API-major change, or intentional unsupported-client block.
4. Prefer additive fields and compatibility shims over lockstep frontend/backend releases; Nevermind does not use feature flags.

## Contract rules

- `/api/v1/*` is stable for supported desktop clients.
- Missing optional desktop headers must not break older clients.
- Unknown JSON fields must be safe for older desktop clients.
- Error shapes, auth semantics, billing behavior, and streaming semantics are part of the contract.
- Backend request identity headers are observability and compatibility metadata, never authentication.
- A backend-only change must not require a not-yet-installed desktop release unless it remains compatible or returns an explicit update requirement.

## Required review questions

- Which released desktop versions can call this route?
- What happens if the desktop does not send the new field/header?
- What happens if the backend returns an unknown field/error?
- Is the change safe under continuous backend deploys?
- Does a new server-side prerequisite reject previously successful requests with existing production configuration, including pricing and provider routes? Unchanged JSON shapes do not establish compatibility.
- Does the user get a palette-safe update/account action if the client is unsupported?
- Do logs identify desktop version, API contract version, route, status, and request ID?

## Verification expectations

For implementation work, add or update contract coverage for:

- compatibility manifest shape
- unsupported-client response shape
- device auth initiation/exchange when changed
- token revoke when changed
- active-model descriptor shape when changed
- AI proxy success/error/rate-limit/billing responses when changed
- streaming response behavior when changed

For new prerequisites, include regression cases using released-client requests and the configured production model/provider combinations, including absent catalog metadata. Mocked pricing success is not evidence that those models are covered. Establish read-only production configuration evidence before a merge that automatically deploys; leave unverified prerequisites blocking readiness rather than requiring users to install a new desktop release.

Use `mise exec -- pnpm` and the repository's CI-only runtime verification policy.

## Output expectations

When reporting compatibility work, include:

- affected desktop callers and backend routes
- whether the change is additive, breaking, or a shim
- supported desktop versions considered
- contract tests or manual verification performed
- residual rollout or sunset risks

---
name: backend-api-compatibility
description: Use when changing or reviewing Nevermind desktop/backend contracts - Astro API routes used by desktop, Electron backend fetches, auth/device login, token revoke, active-model descriptors, AI proxy routes, billing/rate-limit/error shapes, compatibility manifests, feature flags, backend deploy policy, desktop release/update interactions, or any request about keeping frontend and backend in sync.
---

# Backend API Compatibility

Nevermind's Electron desktop app ships on tagged releases while the backend may deploy continuously. Treat the backend as a backwards-compatible service for installed desktop clients.

## Start here

1. Read `src/docs/backend-api-compatibility.md`.
2. Map both sides of the contract before changing code:
   - Desktop callers in `src/electron/nevermind-auth.ts`, `src/electron/ai.ts`, and related main-process flows.
   - Backend routes in `backend/src/pages/api/**` and shared backend libraries in `backend/src/lib/**`.
3. Identify the change type: additive, feature-gated, compatibility shim, API-major change, or intentional unsupported-client block.
4. Prefer compatibility gates and additive fields over lockstep frontend/backend releases.

## Contract rules

- `/api/v1/*` is stable for supported desktop clients.
- Missing optional desktop headers must not break older clients.
- Unknown JSON fields must be safe for older desktop clients.
- Error shapes, auth semantics, billing behavior, and streaming semantics are part of the contract.
- Backend request identity headers are observability and compatibility metadata, never authentication.
- A backend-only change must not require a not-yet-installed desktop release unless it is gated or returns an explicit update requirement.

## Required review questions

- Which released desktop versions can call this route?
- What happens if the desktop does not send the new field/header?
- What happens if the backend returns an unknown field/error?
- Is the change safe under continuous backend deploys?
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

Run package commands through `mise exec pnpm` as required by the repo guidelines.

## Output expectations

When reporting compatibility work, include:

- affected desktop callers and backend routes
- whether the change is additive, gated, breaking, or a shim
- supported desktop versions considered
- contract tests or manual verification performed
- residual rollout or sunset risks

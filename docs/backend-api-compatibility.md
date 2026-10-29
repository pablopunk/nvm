# Backend API compatibility

Nevermind's desktop app and backend do not deploy at the same cadence. Desktop users install tagged Electron releases when they choose to update; the backend may deploy continuously from `main`. The backend must therefore treat every supported desktop release as an active client.

## Product contract

- Backend deploys must be safe for supported installed desktop versions.
- Desktop/backend synchronization is achieved through API contracts, not lockstep releases.
- `/api/v1/*` is the current stable desktop API surface.
- Changes inside an API major version are additive by default.
- Breaking changes require either a new API major version or a staged compatibility gate.
- Unsupported clients should receive an explicit update path, never an unexplained AI/auth failure.

## Supported-client policy

Support at least the latest two minor desktop versions or the last 90 days of released desktop versions, whichever keeps more users covered. Patch releases in a supported minor line remain supported.

Nightly builds use the same API contract and support window. Prerelease versions sort before the corresponding stable version: a minimum of `0.7.0` does not include `0.7.0-nightly.1`. Changes to version comparison must preserve the behavior of supported released clients.

Emergency security blocks may shorten the window, but the backend must return a clear compatibility response that lets desktop render an update prompt.

## Desktop request identity

Desktop backend requests should include non-secret identity metadata: app name, desktop app version, requested API contract version, platform, architecture, and an optional request ID.

These values are for compatibility, observability, and support. They are not authentication and must not replace token/session checks.

For audio transcription, `Idempotency-Key` remains the per-segment billing identity. New desktop clients may additionally send `X-Nevermind-Dictation-Operation-ID` to identify the root recording across segments. Preparation DELETE requests may identify explicit user cancellation with `X-Nevermind-Dictation-Cancellation-Reason: user_cancel`; without that header, cancellation is legacy cleanup and must not tombstone a missing reservation. These optional headers preserve the existing JSON body for installed clients and older backend deployments.

## Compatibility manifest

The backend owns a small manifest that desktop can fetch during startup, sign-in, and AI setup. The manifest should describe the backend deployment, current and supported API majors, minimum supported desktop version, latest known desktop version, and any deprecation or force-update notices.

Desktop should cache the last successful manifest, show cached state immediately, and refresh in place.

## Compatibility errors

Compatibility failures should use a consistent JSON shape with a machine-readable error type, human-readable message, minimum supported desktop version, latest known desktop version when available, update URL, and request ID.

Desktop should translate these responses into palette-safe account/update UI and reuse the existing update actions where possible.

## Feature delivery

Ship features directly with additive contracts or compatibility shims. Nevermind does not use feature flags or staged rollouts. The manifest retains its legacy `features` object with always-on values only for compatibility with released desktop clients.

Server-side kill switches should exist for risky behavior such as model provider changes, streaming transformations, billing enforcement changes, and auth flow changes. `NEVERMIND_KILL_SWITCHES` supports comma-list or JSON boolean switches for `ai_proxy`, `ai_streaming`, `audio_transcription`, and `auth_device`.

## Pricing admission prerequisite

Priced routes admit only from durable pricing snapshots that the scheduled refresh validated, and audio and decision catalogs are read the same way. The `Database migration` workflow seeds and verifies those snapshots (`pnpm -C src/backend db:seed-pricing`) after migrations and before any deploy takes traffic. The gate requires every configured route to admit a current price. If it fails, the previous deployment keeps serving, so a deploy never reaches clients unpriced.

Released desktop clients keep the `pricing_unavailable` error type and the existing `unverified` reason. The `reason: stale` appears past the hard bound, and for a stale price once the day's overrun cap (`PRICING_OVERRUN_DAILY_CAP_CREDITS`) is reached. Stale prices now admit for every tier below that cap, so a free route is serviceable inside the bound and under the cap. `active-model` now includes a free route's estimate when its price is stale, at the same margin its reservation holds. Each of these is additive, so no desktop release is required.

Two changes to which prices admit are not additive. A bundled price stops admitting past its verification window, and an OpenRouter route is priced from its live endpoints rather than its model-level price. The gate refuses a route that has no price under the new rules, so it blocks such a deploy instead of letting clients see the refusal. It does not prove that every such route is still the one clients expect. Compatibility is therefore not established by this document alone: the outstanding production prerequisites in `src/backend/README.md` (CRON_SECRET, cron cadence, and a first gated deploy that logged `"ready": true`) must be confirmed before this change is treated as released-client safe.

## API-major breaking-change criteria

Create a new API major, such as `/api/v2`, only when a backend change cannot be safely represented as an additive field, compatibility shim, or explicit unsupported-client block inside the current major.

A change is API-major breaking when it removes or renames a field used by a supported desktop release, changes an error type/status that desktop handles specially, changes auth or billing semantics, changes model descriptor/provider routing in a way old clients cannot understand, changes streaming framing or termination semantics, or requires desktop to send a new non-optional request field/header.

Before retiring an API major, the backend owner must document the sunset date, verify active client counts are below the supported-client threshold, preserve user-visible update messaging, and keep compatibility errors available until the final sunset window closes. Use `docs/backend-api-sunset-template.md` for this record.

CI runs `scripts/check-backend-api-major.cjs`, which allows no `/api/v2` routes by default and requires a dedicated `docs/backend-api-v2.md` migration/sunset document if a real `/api/v2` route is introduced.

## Contract testing

Every backend route used by desktop is part of the product contract. Contract tests should cover successful responses, error shapes, auth/device flows, active-model descriptors, proxy routes, streaming semantics, rate limits, and unsupported-client responses.

Store stable JSON fixtures under `src/backend/src/fixtures/contracts/<desktop-api-major>/`. Fixtures should represent full response contracts that a supported desktop release depends on, including compatibility manifests and machine-readable error envelopes. Route-level tests may use builders/mocks for volatile fields such as request IDs, random device codes, and streaming bodies, but the response shape must stay stable.

CI runs `scripts/check-backend-contract-fixtures.cjs` and `pnpm -C src/backend test` as part of the root `pnpm test` command. Backend route changes are not ready until fixture validation and route contract tests pass.

When a desktop release is tagged, review the current desktop-used backend routes and add or refresh fixtures for any newly depended-on shape before the tag is pushed. The release commit should state whether contract fixtures changed or why no fixture update was needed.

## Backend contract PR/release checklist

For any PR touching `src/backend/src/pages/api`, `src/backend/src/lib/proxy.ts`, auth/token/device flows, model descriptors, billing/rate-limit responses, or desktop compatibility headers:

- Add or update route-level contract tests for changed success and error shapes.
- Add or update JSON fixtures under `src/backend/src/fixtures/contracts/` when a supported desktop release depends on the shape.
- Verify `mise exec -- pnpm test` locally.
- If tagging a desktop release, note in the release commit whether contract fixtures changed.
- If a supported client cannot be kept compatible, use the API-major or unsupported-client process below.
- Before sunsetting an API major, query `desktop_client_seen` logs for active clients by `client_api_version` and `client_version`.

## Breaking-change checklist

Before changing a backend route used by desktop, decide whether the change is:

1. Additive and safe for older clients.
2. Compatible through an existing desktop version boundary.
3. A new API major version.
4. A compatibility shim that keeps old clients working.
5. An intentional unsupported-client block with update UX.

If none of these apply, do not ship the backend change.

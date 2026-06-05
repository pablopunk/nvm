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

Emergency security blocks may shorten the window, but the backend must return a clear compatibility response that lets desktop render an update prompt.

## Desktop request identity

Desktop backend requests should include non-secret identity metadata: app name, desktop app version, requested API contract version, platform, architecture, and an optional request ID.

These values are for compatibility, observability, support, and feature gating. They are not authentication and must not replace token/session checks.

## Compatibility manifest

The backend owns a small manifest that desktop can fetch during startup, sign-in, and AI setup. The manifest should describe the backend deployment, current and supported API majors, minimum supported desktop version, latest known desktop version, available feature flags, and any deprecation or force-update notices.

Desktop should cache the last successful manifest, show cached state immediately, and refresh in place.

## Compatibility errors

Compatibility failures should use a consistent JSON shape with a machine-readable error type, human-readable message, minimum supported desktop version, latest known desktop version when available, update URL, and request ID.

Desktop should translate these responses into palette-safe account/update UI and reuse the existing update actions where possible.

## Feature rollout

New backend capabilities should be gated by explicit manifest features, not inferred from backend deploy versions. Desktop should ignore unknown features and only enable new behavior when the expected feature is present.

Feature flags may be returned by `GET /api/compatibility` in the `features` object. Backend configuration supports simple comma-list flags and JSON rules with desktop version, user, plan, and rollout constraints. Rollout percentages must be deterministic for a client/user so support can reason about why a user did or did not receive a feature.

Desktop must use `requireNevermindCompatibilityFeature` or `nevermindCompatibilityFeatureEnabled` before relying on new backend-advertised behavior. The backend currently advertises `active_model_descriptor` and `proxy_streaming` by default so desktop can gate dynamic model routing and future streaming behavior explicitly.

Server-side kill switches should exist for risky behavior such as model provider changes, streaming transformations, billing enforcement changes, and auth flow changes. `NEVERMIND_KILL_SWITCHES` supports comma-list or JSON boolean switches for `ai_proxy`, `ai_streaming`, and `auth_device`.

## API-major breaking-change criteria

Create a new API major, such as `/api/v2`, only when a backend change cannot be safely represented as an additive field, optional feature flag, compatibility shim, or explicit unsupported-client block inside the current major.

A change is API-major breaking when it removes or renames a field used by a supported desktop release, changes an error type/status that desktop handles specially, changes auth or billing semantics, changes model descriptor/provider routing in a way old clients cannot understand, changes streaming framing or termination semantics, or requires desktop to send a new non-optional request field/header.

Before retiring an API major, the backend owner must document the sunset date, verify active client counts are below the supported-client threshold, preserve user-visible update messaging, and keep compatibility errors available until the final sunset window closes. Use `src/docs/backend-api-sunset-template.md` for this record.

CI runs `scripts/check-backend-api-major.cjs`, which allows no `/api/v2` routes by default and requires a dedicated `src/docs/backend-api-v2.md` migration/sunset document if a real `/api/v2` route is introduced.

## Contract testing

Every backend route used by desktop is part of the product contract. Contract tests should cover successful responses, error shapes, auth/device flows, active-model descriptors, proxy routes, streaming semantics, rate limits, and unsupported-client responses.

Store stable JSON fixtures under `backend/src/fixtures/contracts/<desktop-api-major>/`. Fixtures should represent full response contracts that a supported desktop release depends on, including compatibility manifests and machine-readable error envelopes. Route-level tests may use builders/mocks for volatile fields such as request IDs, random device codes, and streaming bodies, but the response shape must stay stable.

CI runs `scripts/check-backend-contract-fixtures.cjs` and `pnpm -C backend test` as part of the root `pnpm test` command. Backend route changes are not ready until fixture validation and route contract tests pass.

When a desktop release is tagged, review the current desktop-used backend routes and add or refresh fixtures for any newly depended-on shape before the tag is pushed. The release commit should state whether contract fixtures changed or why no fixture update was needed.

## Backend contract PR/release checklist

For any PR touching `backend/src/pages/api`, `backend/src/lib/proxy.ts`, auth/token/device flows, model descriptors, billing/rate-limit responses, or desktop compatibility headers:

- Add or update route-level contract tests for changed success and error shapes.
- Add or update JSON fixtures under `backend/src/fixtures/contracts/` when a supported desktop release depends on the shape.
- Verify `mise exec -- pnpm test` locally.
- If tagging a desktop release, note in the release commit whether contract fixtures changed.
- If a supported client cannot be kept compatible, use the API-major or unsupported-client process below.
- Before sunsetting an API major, query `desktop_client_seen` logs for active clients by `client_api_version` and `client_version`.

## Breaking-change checklist

Before changing a backend route used by desktop, decide whether the change is:

1. Additive and safe for older clients.
2. Gated by a feature flag or desktop version.
3. A new API major version.
4. A compatibility shim that keeps old clients working.
5. An intentional unsupported-client block with update UX.

If none of these apply, do not ship the backend change.

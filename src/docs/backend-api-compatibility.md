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

Server-side kill switches should exist for risky behavior such as model provider changes, streaming transformations, billing enforcement changes, and auth flow changes.

## Contract testing

Every backend route used by desktop is part of the product contract. Contract tests should cover successful responses, error shapes, auth/device flows, active-model descriptors, proxy routes, streaming semantics, rate limits, and unsupported-client responses.

When a desktop release is tagged, preserve the request/response expectations needed to keep that release supported through the support window.

## Breaking-change checklist

Before changing a backend route used by desktop, decide whether the change is:

1. Additive and safe for older clients.
2. Gated by a feature flag or desktop version.
3. A new API major version.
4. A compatibility shim that keeps old clients working.
5. An intentional unsupported-client block with update UX.

If none of these apply, do not ship the backend change.

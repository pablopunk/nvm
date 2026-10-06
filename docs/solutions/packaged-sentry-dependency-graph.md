# Packaged Sentry dependency graph

## Failure boundary

Nevermind 0.21.1 contained `@sentry/electron`, but the SDK could not load because its `require-in-the-middle` dependency could not resolve `module-details-from-path`. The same failure occurred in the published Linux package and the installed macOS ASAR lacked the required module. A successful source build and source-map upload did not establish runtime delivery.

## Prevention

Use a packager that supports the installed pnpm dependency layout. Verify the final ASAR dependency graph, not only direct SDK files or source imports. Present optional dependencies also need their required graph. Native-architecture CI packages must send safe errors and successful journeys through the real SDK to an isolated loopback collector before publication. Keep test-mode command access narrow and separate from ordinary smoke tests.

## Evidence boundary

The Linux package from `5612986ec1e1b8c7dcc948c85b7f5be4e789e0d7` loaded the SDK in isolated homelab VM 224. The real palette's Report a Problem command produced event `20efd2e3ecd04b3cb15e1b5a4b18a49f`, confirmed through the authenticated Sentry project API on 2026-10-06, with the exact release and support reference `a03d80da888c334969aeb422e2eb4d1b`.

This proves Linux packaged error delivery for that commit, not a later build, a repaired macOS installation, source-map symbolication, or remote transaction availability. Organization transaction queries require additional read permission; HTTP 403 is not evidence that transactions are absent.

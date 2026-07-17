# Desktop Sentry observability

## Current decision

Keep Sentry initialized in the Electron **main process only**. Do not add
renderer-side Sentry initialization or source-map upload yet.

The main process is the right first boundary for the app's startup, lifecycle,
IPC, updater, and uncaught-process errors. It also avoids embedding the Sentry
DSN and SDK into the renderer bundle while the renderer does not have a useful
error-reporting path or source maps.

## Configuration

`src/electron/sentry.ts` reads `SENTRY_DSN_DESKTOP` first, then the legacy
`NEVERMIND_SENTRY_DSN`, before falling back to the existing production DSN.
Release builds should set `SENTRY_DSN_DESKTOP` in their build environment so a
DSN rotation does not require a source change.

The configured production DSN targets Sentry project ID `4511502032502784`,
which is the Electron (`nvm-app`) project. The main-process startup handlers
explicitly send uncaught exceptions and unhandled rejections to that project.

## Backend health monitor

Sentry monitors the backend health endpoint at:

`https://api.nvm.fyi/api/health` (the API base is the origin `https://api.nvm.fyi`; browser clients use relative `/api/...` paths on `https://www.nvm.fyi`).

Monitor: [Backend API health](https://pablopunk.sentry.io/monitors/7795419/?project=4511502032502784&statsPeriod=14d)

## Source maps and renderer errors

The Electron renderer build deliberately sets `sourcemap: false` in
`electron.vite.config.ts`. Uploading source maps without enabling matching
build output would not improve stack traces, so no Sentry upload step is
configured.

Revisit this decision when renderer failures are a material support signal. A
renderer rollout should be one scoped change that:

1. Enables hidden production source maps for the renderer.
2. Uploads them with the exact release/version used by `Sentry.init`.
3. Adds renderer initialization with an explicit privacy review for UI content.
4. Verifies a test renderer exception resolves to original TypeScript in Sentry.

Until then, keep renderer diagnostics in the existing local logging and IPC
error paths, and use main-process Sentry for production crash visibility.

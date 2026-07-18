# Plans

Dependency upgrade plans are tracked in Multica issues. Plan 005 upgraded the
desktop Pi runtime and backend Astro patch chain, with production audits and
packaged-runtime verification recorded in the issue result.

The root lock also pins `ws` 8.21.0 and `protobufjs` 7.6.3 through pnpm
overrides. Both versions satisfy the current upstream ranges declared by the
Pi provider graph; remove these overrides once Pi's `@google/genai` dependency
resolves them without help. The backend lock similarly pins Vite 7.3.5 because
Astro 6.4.8 declares a compatible `^7.3.2` range; remove that override once a
normal backend lock refresh selects a patched Vite.

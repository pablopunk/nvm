# Packaged app size

Nevermind keeps production packages limited to code that can run in the packaged Electron main/preload runtime. Renderer, build, and test packages should not be shipped in production `node_modules` because the renderer is bundled into `dist/renderer` before Electron Builder packages the app.

## Dependency classification

Keep in `dependencies` only packages required by packaged runtime code, native runtime helpers, updater/logging, AI/session runtime, Sentry, and generated-extension validation.

Move to `devDependencies` when a package is only used by:

- renderer source bundled into `dist/renderer`
- Vite/Electron-Vite build config
- tests/type declarations
- local development scripts

Current renderer/build packages that intentionally live in `devDependencies` include:

- `@vitejs/plugin-react`
- `vite`
- `react`
- `react-dom`
- `lucide-react`
- `cmdk`
- `react-markdown`
- `remark-gfm`

Do not add direct AI SDK packages unless Nevermind source imports them directly. AI runtime goes through `@earendil-works/pi-ai` and `@earendil-works/pi-coding-agent`, so direct `@ai-sdk/anthropic`, `@ai-sdk/google`, and `@ai-sdk/openai` dependencies should stay absent unless the source architecture changes.

## Guardrail

`scripts/check-packaged-runtime-imports.cjs` runs in `pnpm test` after `pnpm build`. It scans `dist/main` and `dist/preload` for static imports, dynamic imports, `require(...)`, and string references to packages that must not be used by packaged runtime code.

If main/preload starts depending on a renderer/build/dev-only package, fix the boundary instead of moving that package back to `dependencies`. Renderer code should remain bundled; packaged runtime code should stay explicit about its runtime dependencies.

## Why TypeScript stays packaged

TypeScript is intentionally still a production dependency. The main process uses it to typecheck and validate generated Nevermind extensions before loading them. Do not remove or trim TypeScript without adding packaged extension-validation smoke coverage.

## What not to strip casually

- Keep Sentry: production crash/error visibility is more valuable than the size saving.
- Do not manually prune `@earendil-works/pi-ai` provider dependencies unless the backend/desktop contract guarantees which provider adapters are needed.
- Do not strip Electron framework internals beyond supported Electron Builder options; random Chromium/Electron resource removal can break rendering, webviews, media, signing, or launch.

## Verification for size-related dependency changes

Run:

```sh
mise exec -- pnpm test
CSC_IDENTITY_AUTO_DISCOVERY=false mise exec -- pnpm dist:mac:arm64
```

Then inspect `release/mac-arm64/Nevermind.app/Contents/Resources/app.asar` and confirm removed packages are absent. In the dependency-cleanup pass, `app.asar` dropped from roughly 105 MB to 82 MB while keeping packaged runtime checks and tests green.

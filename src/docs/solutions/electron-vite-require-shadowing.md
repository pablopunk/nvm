# Electron Vite Require Shadowing

## Problem or symptoms

A packaged Nevermind build can fail during Electron main-process startup with:

```text
A JavaScript error occurred in the main process
SyntaxError: Identifier 'require' has already been declared
    at compileSourceTextModule (node:internal/modules/esm/utils:319:16)
```

The crash happens before app initialization, so normal app logging or Sentry capture may not run.

## Context

This surfaced after changing `src/electron/sentry.ts` to load `@sentry/electron/main` through `createRequire(import.meta.url)` so Sentry could be optional or package-safe at startup.

The TypeScript source was valid and `electron-vite build` completed, but the generated ESM main bundle was not parseable.

## What did not work

- Typechecking did not catch the issue because the source module had no duplicate local declaration.
- Building alone did not catch the issue because Electron/Vite emitted the invalid bundle without parsing it as Node would at startup.
- Inspecting source imports was misleading; the duplicate declaration was introduced by generated CommonJS shim code plus the bundled source binding.

## Root cause

Electron/Vite injects a CommonJS shim into the ESM main bundle that declares a top-level `require` binding:

```js
const require = __cjs_mod__.createRequire(import.meta.url)
```

The Sentry module also declared a top-level `require` binding from `createRequire`. In the bundled output that became another top-level `require` declaration in the same module, producing the startup syntax error.

## Fix

Rename the app-owned `createRequire` binding in `src/electron/sentry.ts` to a domain-specific name, such as `requireSentryModule`, instead of using the reserved/common shim name `require`.

Add generated-bundle syntax checks to the normal test path so this class of production-only failure is caught:

```bash
mise exec -- pnpm test
```

The test script now builds and runs:

```bash
node --check dist/main/main.js
node --check dist/preload/preload.cjs
```

## Verification

Run:

```bash
mise exec -- pnpm build
node --check dist/main/main.js
mise exec -- pnpm test
```

The generated `dist/main/main.js` should parse successfully and should not contain a second top-level declaration of `require` from app code.

## Notes for future searches

Keywords: Electron main process, electron-vite, Vite bundle, CommonJS shim, createRequire, require shadowing, Identifier 'require' has already been declared, compileSourceTextModule, Sentry startup crash, packaged build.

# Idempotent Action Tokenization

## Problem

Large extension views could show `Untrusted runExtensionAction action` when a visible action was selected. Open With in a media grid exposed this as a generic Action failed view before the app picker opened.

## Root cause

Extension results are normalized with their extension context and normalized again at the final IPC boundary. Each pass used to replace every valid execution token. A grid with nested preview views could therefore create more than the bounded token registry allowed during one response, which evicted tokens that were still present in the response sent to the renderer.

## Fix

Action registration is idempotent. It preserves an existing token only when the host still owns the token and its stored trusted payload is an exact deep match for the action being normalized. Changed or unknown payloads receive a new token, so renderer and extension input cannot reuse a token for a different privileged action.

## Verification

Run `mise exec -- pnpm typecheck` and `mise exec -- pnpm test`, then open a large media grid and confirm that Open With opens its app picker and launches the selected app.

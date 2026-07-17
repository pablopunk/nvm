# WorkOS AuthKit production login recovery

## Problem or symptoms

Production and Preview sign-in could fail with either `Sign-in expired; please restart.` or a 500 response after a successful Google sign-in. A redirect to the WorkOS AuthKit page alone was not enough to verify either flow.

## Context

The backend stores one-use OAuth state in Upstash Redis, exchanges the WorkOS callback code, and then provisions or loads a local `users` row. Local users are unique by both `workos_user_id` and `email`.

## Root causes

1. Upstash can deserialize JSON stored by `SET` before returning it from `GETDEL`. Parsing that object as a string rejected valid OAuth state.
2. User provisioning only looked up local users by WorkOS ID. When a verified WorkOS identity changed while the same email already existed locally, the fallback insert violated the unique email constraint and returned 500.
3. A Vercel deployment being Ready did not guarantee that the public aliases served it, so an apparent production test could still exercise an older build.

## Fix

- Normalize Redis `GETDEL` values to JSON text before validation and give OAuth state a ten-minute lifetime.
- For an authenticated WorkOS identity, first look up the local user by WorkOS ID; if missing, find the same canonical email and atomically replace only its stale WorkOS ID. If no local email exists, create the user and its initial grant as usual. The authenticated WorkOS email is suitable for this reconciliation because AuthKit verifies email ownership and prevents unsafe identity linking.
- Emit redacted callback-stage logs without codes, states, cookies, or email addresses.
- Before live validation, inspect the deployment aliases and verify `nvm.fyi`, `www.nvm.fyi`, and `api.nvm.fyi` resolve to the target deployment. Use `https://www.nvm.fyi/api/auth/callback` as the non-redirecting production WorkOS callback; retain the apex callback only during migration.

## What did not prove the fix

- Reaching the WorkOS page.
- A synthetic callback with an invalid code.
- A Ready Vercel deployment without checked public aliases.
- Unit and backend tests without a real callback.

## Verification

Run the focused identity-linking regression test, `mise exec -- pnpm -C backend check`, `mise exec -- pnpm -C backend test`, root typecheck, and root tests. Then deploy, verify public aliases, and complete a fresh Google sign-in in production. The required end state is a successful callback and dashboard redirect, not only a 302 to WorkOS.

## Keywords

WorkOS, AuthKit, Google OAuth, callback, `Sign-in expired`, Upstash, Redis, `GETDEL`, `users`, unique email, stale WorkOS ID, Vercel aliases, production login.

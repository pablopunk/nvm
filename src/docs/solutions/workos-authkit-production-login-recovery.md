# WorkOS AuthKit production login recovery

## Problem or symptoms

Production and Preview sign-in could fail with `Authentication is temporarily unavailable`, `Sign-in expired; please restart.`, an invalid redirect URI, or a 500 response after a successful Google sign-in. A redirect to the WorkOS AuthKit page alone was not enough to verify either flow.

## Context

The backend stores one-use OAuth state in Upstash Redis, exchanges the WorkOS callback code, and then provisions or loads a local `users` row. Local users are unique by both `workos_user_id` and `email`.

## Root causes

1. Upstash can deserialize JSON stored by `SET` before returning it from `GETDEL`. Parsing that object as a string rejected valid OAuth state.
2. User provisioning only looked up local users by WorkOS ID. When a verified WorkOS identity changed while the same email already existed locally, the fallback insert violated the unique email constraint and returned 500.
3. A Vercel deployment being Ready did not guarantee that the public aliases served it, so an apparent production test could still exercise an older build.
4. Strict production auth validation requires `PRODUCTION_ORIGIN=https://www.nvm.fyi`, matching public dashboard and Preview gateway origins, and the direct `https://www.nvm.fyi/api/auth/callback` redirect. Missing values, the redirected apex host, or surrounding whitespace fail closed before WorkOS.
5. WorkOS must separately allowlist the exact canonical callback. Correct Vercel configuration cannot repair a missing WorkOS redirect URI.
6. Repeated diagnostics and device attempts share the auth IP rate-limit bucket. A temporary `429` can therefore follow a successful configuration repair until the one-minute window expires.

## Fix

- Normalize Redis `GETDEL` values to JSON text before validation and give OAuth state a ten-minute lifetime.
- For an authenticated WorkOS identity, first look up the local user by WorkOS ID; if missing, find the same canonical email and atomically replace only its stale WorkOS ID. If no local email exists, create the user and its initial grant as usual. The authenticated WorkOS email is suitable for this reconciliation because AuthKit verifies email ownership and prevents unsafe identity linking.
- Emit redacted callback-stage logs without codes, states, cookies, or email addresses.
- Emit redacted signin-stage logs that distinguish configuration, state signing, authorization URL, redirect, and state persistence failures.
- Before live validation, inspect the deployment aliases and verify `nvm.fyi`, `www.nvm.fyi`, and `api.nvm.fyi` resolve to the target deployment. Use `https://www.nvm.fyi/api/auth/callback` as the non-redirecting production WorkOS callback; retain the apex callback only during migration.
- Set canonical public values non-interactively so shell newlines are not stored. Sensitive values cannot be read back from Vercel; re-enter a known value without surrounding whitespace or rotate it after confirming the session impact.
- Add the canonical callback to the production WorkOS environment's redirect allowlist. Do not add a trailing slash.
- Keep disabled signups separate from login authorization. Existing local users, including admins, bypass signup policy after WorkOS authentication; only identities without a local user require open signups or a valid invite.

## What did not prove the fix

- Reaching the WorkOS page.
- Receiving a redirect-URI error from WorkOS.
- A synthetic callback with an invalid code.
- A Ready Vercel deployment without checked public aliases.
- Unit and backend tests without a real callback.

## Verification

Run the focused signin and identity-linking regression tests, `mise exec -- pnpm -C backend check`, `mise exec -- pnpm -C backend test`, root typecheck, and root tests. Then deploy, verify public aliases, confirm signin returns a WorkOS `302` with a correlation cookie, and complete a fresh device login in production. If diagnostics triggered `429`, wait for the one-minute auth window and retry once. The required end state is an authenticated desktop session, not only a 302 to WorkOS.

## Keywords

WorkOS, AuthKit, Google OAuth, callback, redirect URI, `Authentication is temporarily unavailable`, `Sign-in expired`, `429`, Upstash, Redis, `GETDEL`, `users`, unique email, stale WorkOS ID, Vercel aliases, production login.

# Production auth and proxy billing smoke

Use this checklist after an approved production deployment. Automated tests cover
configuration, protocol boundaries, transparent tool-call proxying, and ledger
debits with test-safe values; they cannot prove the real WorkOS tenant, packaged
Electron UI, or production telemetry.

## Automated preflight

Run:

```sh
mise exec -- pnpm -C backend check
mise exec -- pnpm -C backend test
NVM_SMOKE_BASE_URL=https://api.nvm.fyi mise exec -- pnpm smoke:deployed
```

Confirm the deployment version and public aliases point at the intended release.
Do not continue if health, compatibility, or aliases identify a different deploy.

## Manual production check

1. In a fresh browser session, sign in at `https://nvm.fyi` with WorkOS. Pass only
   when the real callback completes and the profile/dashboard loads; merely
   reaching AuthKit is not sufficient.
2. Launch the packaged Electron release against the production backend, sign out
   any cached session, and sign in again. Complete the browser verification page
   and confirm the app consumes the device approval and shows the production
   account.
3. Record the account's credit balance without copying tokens, cookies, or other
   secrets.
4. With a Google-backed model active, ask the app to perform a harmless read-only
   tool call such as `list_extensions`, then return a short streamed response.
   Pass only when the tool actually executes and its result is used; rendered
   pseudo-tool text does not count.
5. Reload the account balance and confirm it decreased by the request's billed
   credits.
6. Find the matching structured `chat_completion` production log entry. Confirm
   `status` is `200`, `provider` is `google`, `input_tokens` and `output_tokens`
   are both greater than zero, `cost_credits` is greater than zero, and the
   packaged client version is present.

Record only the deployment/app version, pass/fail result, request ID, token
counts, billed credits, and balance delta. Never paste credentials or auth query
values into an issue. A missing callback, device handoff, real tool execution,
debit, or non-zero usage entry leaves the production smoke incomplete.

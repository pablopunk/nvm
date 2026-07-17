# WorkOS staging Magic Auth E2E

This protected journey exercises Nevermind's real `/api/auth/signin` → hosted
AuthKit → `/api/auth/callback` → `nvm_session` → `/api/me` path twice. It uses a
fresh MailSlurp inbox and user for every run, then deletes the local and WorkOS
staging users and the inbox. Ordinary PR tests never call WorkOS or MailSlurp.

WorkOS calls this passwordless method **Magic Auth**. Magic Link is deprecated.
Magic Auth sends a six-digit, one-use code that expires after ten minutes. See
the [Magic Auth guide](https://workos.com/docs/authkit/magic-auth) and
[staging environment guide](https://workos.com/docs/authkit/environments).

## One-time staging setup

1. In the WorkOS dashboard, select the Nevermind **staging** environment. Enable
   Magic Auth under Authentication. Never copy production users, keys, or
   configuration into this lane.
2. Deploy a fixed HTTPS test origin backed only by an isolated test Postgres
   database and a separate state Redis resource. Configure its exact callback as
   `https://<fixed-staging-host>/api/auth/callback` in WorkOS staging and set the
   deployment's `WORKOS_REDIRECT_URI` to that exact value.
3. Choose a unique non-secret environment ID. Set
   `GATEWAY_STATE_NAMESPACE=nvm:magic-auth-e2e:<environment>:v1`,
   `NVM_MAGIC_AUTH_ENVIRONMENT_ID`, `NVM_MAGIC_AUTH_WORKOS_ENV=staging`, and a
   staging-only `NVM_MAGIC_AUTH_PROBE_SECRET` on that deployment. Its Redis
   credential must not reach production Redis. Store the environment ID at
   `<namespace>:environment` in that Redis resource. Set a
   `magic_auth_e2e_environment_id=<environment ID>` row and a
   persisted `app_settings` row `signups_enabled=true`; the runner checks this so
   an invite-gated failure cannot be mistaken for a provider failure.
4. Create a MailSlurp API key dedicated to this workflow. The runner creates a
   new inbox per run, records the send timestamp before submission, accepts only
   a newly delivered message addressed to that exact inbox, parses the code in
   memory, deletes each message, and deletes the inbox during cleanup.
5. Create the protected GitHub environment `workos-staging`. Limit deployment
   branches to `main`, require reviewers if appropriate, and add the following.

Environment variables:

- `NVM_MAGIC_AUTH_BASE_URL` — the fixed HTTPS staging origin.
- `NVM_MAGIC_AUTH_CALLBACK_URL` — the exact origin plus
  `/api/auth/callback`.
- `NVM_MAGIC_AUTH_ENVIRONMENT_ID` — the exact sentinel stored in the isolated
  database and Redis resource.
- `NVM_MAGIC_AUTH_REDIS_NAMESPACE` — the exact deployment namespace,
  `nvm:magic-auth-e2e:<environment>:v1`.
- `NVM_MAGIC_AUTH_WORKOS_ENV=staging`.

Environment secrets:

- `NVM_MAGIC_AUTH_DATABASE_URL` — isolated test database only.
- `NVM_MAGIC_AUTH_PROBE_SECRET` — staging-only bearer secret shared with the
  controlled deployment probe.
- `WORKOS_API_KEY` — WorkOS staging `sk_test_...` key only.
- `MAILSLURP_API_KEY` — dedicated test-mailbox key.

The controlled deployment separately needs its staging `WORKOS_CLIENT_ID`,
`WORKOS_COOKIE_PASSWORD`, isolated `DATABASE_URL`, `GATEWAY_STATE_KEY`, the
probe/environment settings above, and
isolated `GATEWAY_STATE_REDIS_URL` / `GATEWAY_STATE_REDIS_TOKEN`. Do not place
production credentials, production database URLs, or production Redis tokens in
the GitHub environment.

## Run and verify

Deterministic coverage runs in ordinary CI:

```sh
mise exec -- pnpm -C backend check
mise exec -- pnpm -C backend test
mise exec -- pnpm -C backend test:postgres-integration
```

After the staging deployment and GitHub environment are configured, dispatch
`WorkOS Magic Auth staging E2E` from the default branch. The job is serialized
and also runs weekly from `main`. The runner performs two independent hosted
sign-ins for the same fresh address and asserts:

- `nvm_session` is `HttpOnly`, `Secure`, `SameSite=Lax`, and scoped to `/` on
  the staging host;
- `/api/me` returns the expected address;
- one local user and one initial free-credit grant exist after both sign-ins.

The job retains no screenshots, video, traces, or browser artifacts. It never
prints authorization codes, OAuth state, Magic Auth codes, cookies, callback
URLs, email bodies, or credentials. A failure reports only the journey/job step;
inspect WorkOS, MailSlurp, and deployment dashboards under their own access
controls. Confirm cleanup removed the local row, WorkOS staging user, messages,
and inbox. Run the protected workflow twice before marking PAB-47 complete.

## Local opt-in command

With the same staging-only variables and secrets exported locally:

```sh
mise exec -- pnpm exec playwright install chromium
mise exec -- pnpm -C backend test:magic-auth-e2e
```

The runner fails before opening a browser if the origin is not HTTPS, the
callback is not exact, the WorkOS key is not staging, the database/Redis
sentinels do not match the protected environment, or the Redis namespace is not
explicitly E2E-scoped.

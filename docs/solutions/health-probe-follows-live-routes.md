# Health probe followed a legacy provider instead of live routes

## Problem or symptoms

`https://api.nvm.fyi/api/health` returned 503 with `upstream: false` while production
served AI traffic through OpenRouter. The probe failed because `OPENCODE_API_KEY` is
absent from the Production environment. Nightly readiness, which reads
`upstreamReady`, stayed blocked for the same reason.

## Root cause

The health route probed one provider, `getActiveProvider()`, which reads the
`active_provider` setting and falls back to `opencode_zen`. Live requests do not use
that provider. The AI proxy resolves the `free-*` and `pro-*` slots through
`getModelRoute`, which can fall back to legacy slots, then applies the enabled
failover chain for the primary model. The audio and decision routes resolve their own
providers. The probe therefore measured a provider that carried no traffic and missed
the providers that did.

## Fix

`src/backend/src/lib/upstream-route-verdicts.ts` resolves each live route the way the
request path does. It applies the proxy's refusals and skips:

- A primary without a key or without pricing refuses the route before any failover.
- A failover link is skipped when its key is missing, its wire format differs from the
  primary model's, it has no pricing, or its route breaker is tripped. Skipped links are
  not probed or counted. Breakers are durable: settlement overruns trip them until an
  admin clears them.
- The `ai_proxy` kill switch removes the tiered and decision routes, `ai_failover`
  removes the chain, and `audio_transcription` removes audio. A disabled OpenRouter
  provider refuses audio, as the endpoint does, and a tripped audio breaker refuses
  audio.
- An unsupported audio or decision provider, or a decision model with a charged output,
  is unserviceable.

`src/backend/src/lib/upstream-probe.ts` probes each provider once. Each provider's
readiness uses its own auth semantics from `getUpstreamProbeSpec` in `upstream.ts`:

- OpenRouter's `/models` answers anonymously, so its key is verified on `/key`.
- OpenCode also answers anonymously for legacy keyless access, but it refuses an
  invalid bearer with 401. A 2xx with the configured key therefore shows the key is
  accepted.
- OpenAI and Anthropic refuse anonymous catalog requests with 401, and Google refuses
  them with 403, so a 2xx from their catalogs proves the key.
- Google lists models under `models`. Every other catalog uses `data`.

Liveness keeps its released rule: 401, 403, 404, and 429 still count as reachable, and
5xx or no answer does not. A route is live when any of its candidates answers.
Liveness covers only the tiered chat routes. A missing, disabled, or refused audio or
decision route never flips `upstream`, so a feature outside chat cannot 503 a
deployment whose chat works. A route is ready when every candidate has a 2xx catalog
and an accepted key, and every route, optional ones included, is serviceable.

`src/backend/src/lib/upstream-readiness.ts` runs at most one evaluation per instance,
and it is never abandoned. Concurrent callers and callers after a timeout join it. A
completed result is cached for 30 seconds when ready and 5 seconds otherwise. A
caller waits at most 8 seconds. If the wait ends first, readiness is unknown (false),
and liveness carries the last completed answer for up to 60 seconds. A timed-out wait
caches nothing, and a failed evaluation waits 5 seconds before it is retried. The
database check is not cached.

Route resolution isolates each route's errors. A breaker or failover lookup that
cannot be read leaves its route serviceable but unverified. An unexpected error
becomes that route's failure and does not abort the others.

## Verification notes

Read-only evidence gathered on 2026-10-28 with public GETs, before the change:

- Production `https://api.nvm.fyi/api/health` returned 503 with
  `{"ok":false,"db":true,"upstream":false,"version":"77fff17"}`. The response has
  no `commit` or `upstreamReady` field, so the deployment predates the readiness contract
  and the nightly gate stays closed.
- `vercel env ls production` listed `OPENROUTER_API_KEY` and `OPENCODE_BASE_URL`, and
  did not list `OPENCODE_API_KEY`.
- `app_settings` set `active_provider` to `opencode_zen`. All four tiered route rows,
  `audio_model_route`, and `decision_model_route` named `openrouter`. The
  `model_providers` failover rows keyed to other model IDs, so they never matched the
  primary model.
- The configured audio model `nvidia/parakeet-tdt-0.6b-v3` appears in OpenRouter's
  transcription catalog with duration pricing. The configured decision model
  `typesafe/jev-1.14` appears in OpenRouter's decision catalog with zero completion
  price. Neither appears in `/models`, which is why readiness reads those catalogs, as
  requests do.
- OpenRouter `/models` returned 200 anonymously. OpenRouter `/key` returned 401 for an
  invalid key.
- OpenCode `/models` returned 200 anonymously, and 401 `Invalid credential` for a bogus
  bearer. OpenAI and Anthropic `/models` returned 401 anonymously, and Google `/models`
  returned 403 anonymously.
- `vercel env pull` returns an 11-character placeholder for Sensitive variables,
  including `OPENROUTER_API_KEY`. The production key was therefore not verified from
  this environment, and the placeholder was not a meaningful check.

Test coverage, CI-only per repository policy and not run locally:

- `src/backend/src/lib/upstream-route-verdicts.test.ts` covers the proxy's skips and
  refusals, kill switches, a disabled audio provider as an explicit refusal, missing
  optional routes, unreadable breakers, unsupported providers, and transient lookup
  failures.
- `src/backend/src/lib/upstream-probe.test.ts` covers status semantics, the `/key`
  check, Google's catalog shape, the byte bound, and missing keys.
- `src/backend/src/lib/upstream-readiness.test.ts` covers the liveness and readiness
  rules, including optional routes, single-flight, a cold evaluation that outlasts the
  health wait, carried liveness and its window, the TTLs, failed-evaluation retry,
  reset, and the wait deadline.
- `src/backend/src/pages/api/health.test.ts` covers the endpoint, including missing
  optional routes, a disabled audio provider, and the live database check on a cached
  upstream result.

Deferred: CI runtime verification, and an authenticated production smoke of the
OpenRouter key and of the audio and decision models after deployment. Readiness does
not prove credit balance, rate limits, or that an inference call succeeds.

# Upstash Connector

Verified skill search results: `upstash/skills@upstash`, `upstash/redis-js@redis-js`, and Upstash Context7 docs skills. The `upstash/skills@upstash` listing did not expose a SKILL.md in search output, so verify commands against installed skill/docs before using them.

Use this connector for rate-limit failures, Redis REST/KV availability, missing rate-limit enforcement, and cache/key-prefix incidents.

## Operational flow

1. Start from Axiom: `ratelimit_disabled`, `rate_limited`, Redis/Upstash errors, 429 responses, or unexpected absence of limits.
2. Determine if env is missing vs Upstash outage: this app intentionally disables rate limits when REST URL/token env is absent.
3. Verify current Upstash REST/Redis commands through the Upstash skill/docs before running mutations.
4. Inspect keys/prefixes read-only first; never delete or reset limits without confirmation.

## Nevermind gotchas

- Missing `UPSTASH_REDIS_REST_URL`/token or Vercel KV-compatible env disables limiting by design.
- 429 responses should include rate-limit scope and retry-after.
- Confirm before deleting keys, flushing databases, resetting limits, rotating tokens, or changing Upstash project settings.

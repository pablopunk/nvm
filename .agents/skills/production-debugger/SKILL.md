---
name: production-debugger
description: Use when a deployed Vercel app misbehaves — 5xx on routes that work locally, broken OAuth/auth callback, suspect env-var values, or any "check prod logs" / "why is prod failing" request. Covers Vercel-specific gotchas accumulated while bringing the Nevermind backend live on `www.nvm.fyi` + `api.nvm.fyi`.
---

# Production Debugger

Use this skill when prod-only failures need investigation on the Nevermind Vercel deploy. Local repro is the first move when feasible; this skill captures the gotchas that bit us in prod and would otherwise eat hours.

## Triggers

- 5xx on a route that returns 2xx in `pnpm dev`
- Auth flow lands on an error page (`/redirect-uri-invalid`, 500 on `/api/auth/callback`)
- Suspect env-var drift between local `.env` and Vercel
- Request to "check prod logs", "tail Vercel", or "see what's failing"

## Workflow

1. **Reproduce on a built URL, not the alias.** Hit the canonical host directly with `curl -sI` to see status + headers (especially `Location` for redirect chains).
2. **Probe constructed URLs for paste artefacts.** When env vars feed into outbound URLs or headers, fetch an endpoint that emits them (e.g. `/api/auth/signin` redirecting to the WorkOS authorize URL) and grep for `%0A` / stray whitespace. Trailing `\n` from dashboard paste is the single most common Vercel env-var bug.
3. **If env-var corruption is suspected, rewrite via CLI, not the dashboard:**
   ```bash
   printf '%s' "$VAL" | vercel env add NAME production
   ```
   Then redeploy. `vercel env rm NAME production --yes` first if it already exists.
4. **Redeploys without a git push:** dashboard "Redeploy" action, or `POST /v13/deployments` with the existing deployment's source ref. `vercel --prod` from the repo root fails when project root is a subdir (`<root>/<configured-root>` is searched); either run from the configured root, or skip the CLI for this.
5. **Runtime logs from the CLI are unreliable in non-interactive shells.** Use the API instead:
   ```bash
   TOKEN=$(python3 -c 'import json;print(json.load(open(f"{__import__(\"os\").path.expanduser(\"~\")}/Library/Application Support/com.vercel.cli/auth.json\"))[\"token\"])')
   curl -s -H "Authorization: Bearer $TOKEN" \
     "https://api.vercel.com/v3/deployments/$DEPLOYMENT_ID/events?direction=backward&limit=200&teamId=$TEAM_ID"
   ```
   `v3/.../events` returns build + runtime events with `type` (`stdout`/`stderr`) and `payload.text`.
6. **Verify after each fix:** rehit the failing endpoint with `curl -sI` and confirm the upstream URL/headers no longer leak the bug.

## Vercel-specific gotchas (durable)

- **OAuth callback URLs must target the canonical (non-redirecting) host.** If the apex is configured as a 308 → `www`, register the redirected-to host with the auth provider; OAuth servers do not follow 308s with state cookies.
- **`vercel env ls production` shows `Encrypted` — don't trust the dashboard rendering for whitespace.** When in doubt, rewrite via stdin (`printf '%s'` strips the newline; `echo` does not).
- **WorkOS does not expose redirect-URI management via public REST API.** Adding new callback URLs is dashboard-only — don't waste time probing endpoints. Hand off to the user.
- **Build hash drives the `__drizzle_migrations` lookup.** If `pnpm build` succeeded, the `tsx scripts/migrate.ts` step succeeded — migration failure fails the build fast.

## Rules

- Do not run destructive Vercel operations (`vercel project rm`, env removals for unknown vars) without confirming first.
- Do not commit `.env` or paste raw secret values into chat output. The local `.env` and `~/.zshrc.d/01-secrets.sh` are the canonical sources; Vercel env vars are derived from them.
- When you change a Vercel env var, always trigger a redeploy — env edits do not retroactively rebuild.
- Prefer the API for log queries inside scripts; the CLI streamer is for human interactive use only.

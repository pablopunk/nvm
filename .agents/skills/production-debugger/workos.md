# WorkOS Connector

Verified skill search result: `workos/skills@workos` (`https://skills.sh/workos/skills/workos`). Related: `workos/skills@workos-authkit-base`.

Use this connector for WorkOS AuthKit failures, redirect URI errors, callback issues, user/session problems, and desktop device auth symptoms.

## Guardrail

The WorkOS skill explicitly warns against fabricating CLI commands or dashboard paths. Use the WorkOS skill/reference/docs if installed; otherwise verify against official WorkOS docs/API before giving a command or dashboard instruction.

## Operational flow

1. Start from Sentry/Axiom evidence: callback status, redirect error, WorkOS exception, cookie/session failure, or device-auth exchange failure.
2. Inspect the actual redirect `Location` for `redirect_uri`, host, scheme, `%0A`, `%20`, and state/return target issues without leaking cookies.
3. Confirm WorkOS allowed callback URLs in the dashboard/API only through verified WorkOS guidance.
4. For desktop auth, correlate initiate/approve/exchange statuses with Axiom logs and desktop `nevermind.log`.

## Nevermind gotchas

- Callback URL must be the canonical non-redirecting host; OAuth servers do not follow app redirects with state cookies.
- WorkOS redirect URI management has historically been dashboard-oriented; verify current capabilities with WorkOS docs/skill before claiming API support.
- Do not expose cookies, authorization codes, sealed session values, or user PII unless explicitly needed and redacted.

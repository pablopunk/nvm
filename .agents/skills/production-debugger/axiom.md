# Axiom Connector

Verified skill search result: `axiomhq/skills@axiom-sre` (`https://skills.sh/axiomhq/skills/axiom-sre`). Its key rule applies here: never guess field names or conclusions; query schema/data first.

Use this connector for Vercel runtime logs. Vercel drains production logs to Axiom, so Axiom is the primary backend runtime evidence source.

## Install/connect and discover

Axiom CLI can be installed on macOS with Homebrew:

```bash
brew install --cask axiomhq/tap/axiom
```

The Vercel app env does not necessarily include Axiom credentials because Vercel log drains are integration-managed. If `axiom auth status` says no deployments are configured, run `axiom auth login` or provide an Axiom API token/org out of band; do not expect `vercel env pull` to reveal it.

```bash
axiom version
axiom auth status
axiom dataset list
axiom query "['DATASET'] | getschema"
axiom query "['DATASET'] | where _time > ago(15m) | limit 5" -f json
```

If multiple datasets exist, inspect schemas/recent rows and pick the Vercel drain dataset. Do not assume field names. Learned for this app: Axiom org `pablopunk` has a `vercel` dataset with fields like `request.path`, `request.host`, `request.id`, `request.statusCode`, `vercel.projectName`, and `vercel.projectId`, but it may not include the current Nevermind Vercel project. Verify with `where ['vercel.projectId'] == PROJECT_ID` before claiming log absence/presence.

## Query patterns

Adapt these after `getschema`.

```bash
# recent errors
axiom query "['DATASET'] | where _time > ago(1h) | where level == 'error' or status >= 500 or message contains 'error' | sort by _time desc | limit 50" -f json

# request correlation
axiom query "['DATASET'] | where _time > ago(2h) | where message contains 'REQUEST_ID' or request_id == 'REQUEST_ID' or requestId == 'REQUEST_ID' | sort by _time asc | limit 100" -f json

# host/route
axiom query "['DATASET'] | where _time > ago(1h) | where host contains 'api.nvm.fyi' and path contains '/api/health' | sort by _time desc | limit 100" -f json

# blast radius
axiom query "['DATASET'] | where _time > ago(1h) | summarize count() by status, path | sort by count_ desc | limit 20" -f json
```

Report exact timestamps, counts, route/host/status/deployment, and correlated request ids. Redact secrets, cookies, auth headers, prompts, request bodies, and payload dumps.

# Vercel Connector

Skill search for `vercel` did not reveal a production-ops Vercel connector; results were mostly React/browser/design skills. Use Axiom for Vercel runtime logs, and use Vercel CLI/API for deployment, build, env, domain, and redeploy evidence.

## Runtime logs

Read `axiom.md` first: Vercel production runtime logs drain to Axiom.

## Deployment/build fallback

Use Vercel API events when you need deployment/build metadata or Axiom is unavailable. If `backend/.vercel/project.json` exists, read project/org ids from it but never commit it.

```bash
TOKEN=$(node -e 'const fs=require("fs"),os=require("os"),path=require("path"); const ps=[path.join(os.homedir(),"Library/Application Support/com.vercel.cli/auth.json"),path.join(os.homedir(),".local/share/com.vercel.cli/auth.json")]; for (const p of ps) if (fs.existsSync(p)) { console.log(JSON.parse(fs.readFileSync(p,"utf8")).token); process.exit(0) } process.exit(1)')
PROJECT_ID=$(jq -r .projectId backend/.vercel/project.json)
TEAM_ID=$(jq -r .orgId backend/.vercel/project.json)
curl -sS -H "Authorization: Bearer $TOKEN" \
  "https://api.vercel.com/v6/deployments?projectId=$PROJECT_ID&teamId=$TEAM_ID&target=production&limit=5" | jq '.deployments[] | {uid,url,state,createdAt,meta}'
```

## Env/domain rules

- To retrieve production env locally for debugging, use `umask 077` and `cd backend && vercel env pull .vercel/.env.production.local --environment=production --yes`; never print or commit the pulled file.
- `vercel env ls production` shows `Encrypted`; it cannot prove pasted whitespace.
- Env edits require redeploys.
- Confirm before env removals, secret changes, domain changes, project changes, or redeploys.
- Compare `*.vercel.app`, `api.nvm.fyi`, `nvm.fyi`, and `www.nvm.fyi` when alias/domain behavior is suspect.

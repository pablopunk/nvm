import assert from 'node:assert/strict';
import { chromium, type Browser, type BrowserContext, type Page } from 'playwright';
import { Pool } from 'pg';

type Inbox = { id: string; emailAddress: string };
type ReceivedEmail = { id: string; to?: string[]; body?: string; subject?: string; createdAt?: string };

const required = [
  'NVM_MAGIC_AUTH_BASE_URL',
  'NVM_MAGIC_AUTH_CALLBACK_URL',
  'NVM_MAGIC_AUTH_DATABASE_URL',
  'NVM_MAGIC_AUTH_ENVIRONMENT_ID',
  'NVM_MAGIC_AUTH_PROBE_SECRET',
  'NVM_MAGIC_AUTH_REDIS_NAMESPACE',
  'NVM_MAGIC_AUTH_WORKOS_ENV',
  'WORKOS_API_KEY',
  'MAILSLURP_API_KEY',
] as const;

function configuration() {
  const missing = required.filter((name) => !process.env[name]);
  if (missing.length) throw new Error(`Magic Auth E2E configuration missing: ${missing.join(', ')}`);
  const base = new URL(process.env.NVM_MAGIC_AUTH_BASE_URL!);
  const callback = new URL(process.env.NVM_MAGIC_AUTH_CALLBACK_URL!);
  const databaseUrl = process.env.NVM_MAGIC_AUTH_DATABASE_URL!;
  const environmentId = process.env.NVM_MAGIC_AUTH_ENVIRONMENT_ID!;
  const redisNamespace = process.env.NVM_MAGIC_AUTH_REDIS_NAMESPACE!;
  if (base.protocol !== 'https:' || callback.toString() !== new URL('/api/auth/callback', base).toString()) {
    throw new Error('Magic Auth E2E requires a fixed HTTPS staging origin and its exact callback URI');
  }
  if (process.env.NVM_MAGIC_AUTH_WORKOS_ENV !== 'staging' || !process.env.WORKOS_API_KEY!.startsWith('sk_test_')) {
    throw new Error('Magic Auth E2E requires WorkOS staging credentials');
  }
  if (!/^nvm:magic-auth-e2e:[a-z0-9_-]+:v\d+$/i.test(redisNamespace)) {
    throw new Error('Magic Auth E2E requires an isolated nvm:magic-auth-e2e:* Redis namespace');
  }
  return {
    base,
    databaseUrl,
    environmentId,
    redisNamespace,
    probeSecret: process.env.NVM_MAGIC_AUTH_PROBE_SECRET!,
    workosApiKey: process.env.WORKOS_API_KEY!,
    mailslurpApiKey: process.env.MAILSLURP_API_KEY!,
  };
}

class JourneyStepError extends Error {}

async function step<T>(label: string, action: () => Promise<T>): Promise<T> {
  try {
    return await action();
  } catch {
    throw new JourneyStepError(`Magic Auth E2E failed at ${label}.`);
  }
}

async function mailslurp<T>(path: string, apiKey: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`https://api.mailslurp.com${path}`, {
    ...init,
    headers: { accept: 'application/json', 'x-api-key': apiKey, ...init?.headers },
  });
  if (!response.ok) throw new Error('mailbox API request failed');
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

async function createInbox(apiKey: string): Promise<Inbox> {
  const inbox = await mailslurp<Inbox>('/inboxes/withDefaults', apiKey, { method: 'POST' });
  if (!inbox.id || !/^[^@\s]+@[^@\s]+$/.test(inbox.emailAddress)) throw new Error('mailbox API returned an invalid inbox');
  return inbox;
}

async function waitForCode(inbox: Inbox, apiKey: string, notBefore: number): Promise<{ emailId: string; code: string }> {
  const query = new URLSearchParams({ inboxId: inbox.id, timeout: '120000', unreadOnly: 'true' });
  const email = await mailslurp<ReceivedEmail>(`/waitForLatestEmail?${query}`, apiKey);
  const recipients = email.to?.map((item) => item.toLowerCase()) ?? [];
  const receivedAt = email.createdAt ? Date.parse(email.createdAt) : Number.NaN;
  if (!email.id || !recipients.includes(inbox.emailAddress.toLowerCase()) || !Number.isFinite(receivedAt) || receivedAt < notBefore - 5_000) {
    throw new Error('mailbox did not return the current run recipient');
  }
  const content = `${email.subject ?? ''}\n${email.body ?? ''}`;
  const match = content.match(/(?:verification|one[- ]time|magic auth|code)[^0-9]{0,50}([0-9]{6})/i);
  if (!match?.[1]) throw new Error('mailbox message did not contain a six-digit code');
  return { emailId: email.id, code: match[1] };
}

async function enterEmail(page: Page, email: string) {
  const field = page.locator('input[type="email"], input[name="email"]').first();
  await field.waitFor({ state: 'visible', timeout: 30_000 });
  await field.fill(email);
  await page.getByRole('button', { name: /continue|sign in|log in|send code/i }).first().click();
}

async function enterCode(page: Page, code: string) {
  const combined = page.locator('input[autocomplete="one-time-code"], input[name*="code" i]').first();
  if (await combined.isVisible().catch(() => false)) {
    await combined.fill(code);
  } else {
    const digits = page.locator('input[inputmode="numeric"]');
    await digits.first().waitFor({ state: 'visible', timeout: 30_000 });
    assert.ok((await digits.count()) >= 6, 'AuthKit code input is unavailable');
    for (let index = 0; index < 6; index += 1) await digits.nth(index).fill(code[index]!);
  }
  const submit = page.getByRole('button', { name: /continue|verify|sign in|log in/i }).first();
  if (await submit.isVisible().catch(() => false)) await submit.click();
}

async function signIn(context: BrowserContext, base: URL, inbox: Inbox, apiKey: string) {
  const page = await context.newPage();
  await step('hosted AuthKit navigation', () => page.goto(new URL('/api/auth/signin', base).toString(), { waitUntil: 'domcontentloaded' }));
  const notBefore = Date.now();
  await step('Magic Auth code request', () => enterEmail(page, inbox.emailAddress));
  const { emailId, code } = await step('test inbox delivery', () => waitForCode(inbox, apiKey, notBefore));
  await step('test message cleanup', () => mailslurp<unknown>(`/emails/${encodeURIComponent(emailId)}`, apiKey, { method: 'DELETE' }));
  await step('Magic Auth code submission', () => enterCode(page, code));
  await step('Nevermind callback', () => page.waitForURL((url) => url.origin === base.origin && !url.pathname.startsWith('/api/auth/callback'), { timeout: 60_000 }));

  await step('session cookie assertion', async () => {
    const cookies = await context.cookies(base.origin);
    const session = cookies.find((cookie) => cookie.name === 'nvm_session');
    assert.ok(session, 'nvm_session cookie is missing');
    assert.equal(session.httpOnly, true);
    assert.equal(session.secure, true);
    assert.equal(session.sameSite, 'Lax');
    assert.equal(session.domain.replace(/^\./, ''), base.hostname);
    assert.equal(session.path, '/');
  });
  await step('/api/me assertion', async () => {
    const me = await context.request.get(new URL('/api/me', base).toString());
    assert.equal(me.status(), 200, '/api/me did not authenticate');
    assert.equal((await me.json()).email.toLowerCase(), inbox.emailAddress.toLowerCase());
  });
  await page.close();
}

async function verifyDeploymentIsolation(config: ReturnType<typeof configuration>, pool: Pool) {
  const marker = await pool.query<{ value: string }>("select value from app_settings where key = 'magic_auth_e2e_environment_id'");
  assert.equal(marker.rows[0]?.value, config.environmentId);
  const response = await fetch(new URL('/api/health/magic-auth-e2e', config.base), {
    headers: { authorization: `Bearer ${config.probeSecret}` },
  });
  assert.equal(response.status, 200);
  const body = await response.json() as { ok?: boolean; environmentId?: string; namespace?: string };
  assert.deepEqual(body, { ok: true, environmentId: config.environmentId, namespace: config.redisNamespace });
}

async function listWorkosUsers(email: string, apiKey: string): Promise<string[]> {
  const url = new URL('https://api.workos.com/user_management/users');
  url.searchParams.set('email', email);
  const response = await fetch(url, { headers: { authorization: `Bearer ${apiKey}` } });
  if (!response.ok) throw new Error('WorkOS user lookup failed');
  const body = await response.json() as { data?: { id?: string; email?: string }[] };
  return (body.data ?? [])
    .filter((user) => user.id && user.email?.toLowerCase() === email.toLowerCase())
    .map((user) => user.id!);
}

async function localProvisioning(pool: Pool, email: string) {
  const policy = await pool.query<{ value: string }>("select value from app_settings where key = 'signups_enabled'");
  assert.equal(policy.rows[0]?.value, 'true', 'staging E2E requires signups_enabled=true');
  const result = await pool.query<{ id: string; workos_user_id: string }>(
    `select u.id, u.workos_user_id
       from users u
      where lower(u.email) = lower($1)`,
    [email],
  );
  assert.equal(result.rowCount, 1, 'Magic Auth must provision exactly one local user');
  const grants = await pool.query(
    "select id from credit_ledger where user_id = $1 and reason = 'grant_free_monthly'",
    [result.rows[0]!.id],
  );
  assert.equal(grants.rowCount, 1, 'Magic Auth must create exactly one initial grant');
  return result.rows[0]!;
}

async function run() {
  const config = configuration();
  const pool = new Pool({ connectionString: config.databaseUrl, max: 1 });
  let browser: Browser | undefined;
  let inbox: Inbox | undefined;
  let failure: Error | undefined;
  try {
    await step('deployment datastore isolation probe', () => verifyDeploymentIsolation(config, pool));
    browser = await step('browser startup', () => chromium.launch({ headless: true }));
    const policy = await pool.query<{ value: string }>("select value from app_settings where key = 'signups_enabled'");
    assert.equal(policy.rows[0]?.value, 'true', 'staging E2E requires signups_enabled=true');
    inbox = await step('test inbox creation', () => createInbox(config.mailslurpApiKey));
    assert.equal((await pool.query('select count(*)::int as count from users where lower(email) = lower($1)', [inbox.emailAddress])).rows[0].count, 0);
    for (let attempt = 0; attempt < 2; attempt += 1) {
      const context = await browser.newContext({ recordVideo: undefined });
      try {
        await signIn(context, config.base, inbox, config.mailslurpApiKey);
      } finally {
        await context.close();
      }
      await step('idempotent local provisioning assertion', () => localProvisioning(pool, inbox!.emailAddress));
    }
    console.log('Magic Auth staging E2E passed twice with idempotent provisioning and protected sessions.');
  } catch (error) {
    failure = error instanceof JourneyStepError ? error : new JourneyStepError('Magic Auth E2E failed at staging preconditions.');
  } finally {
    const cleanupFailures: string[] = [];
    let workosUserIds: string[] = [];
    if (inbox) {
      try {
        const localUsers = await pool.query<{ workos_user_id: string }>(
          'select workos_user_id from users where lower(email) = lower($1)',
          [inbox.emailAddress],
        );
        workosUserIds.push(...localUsers.rows.map((user) => user.workos_user_id));
      } catch {
        cleanupFailures.push('local user discovery');
      }
      try {
        workosUserIds.push(...await listWorkosUsers(inbox.emailAddress, config.workosApiKey));
      } catch {
        cleanupFailures.push('WorkOS user discovery');
      }
      try {
        await pool.query('delete from users where lower(email) = lower($1)', [inbox.emailAddress]);
      } catch {
        cleanupFailures.push('local user cleanup');
      }
      for (const workosUserId of new Set(workosUserIds)) {
        try {
          const response = await fetch(`https://api.workos.com/user_management/users/${encodeURIComponent(workosUserId)}`, {
            method: 'DELETE',
            headers: { authorization: `Bearer ${config.workosApiKey}` },
          });
          if (!response.ok && response.status !== 404) throw new Error('WorkOS cleanup failed');
        } catch {
          cleanupFailures.push('WorkOS user cleanup');
        }
      }
      try {
        await mailslurp<unknown>(`/inboxes/${encodeURIComponent(inbox.id)}`, config.mailslurpApiKey, { method: 'DELETE' });
      } catch {
        cleanupFailures.push('test inbox cleanup');
      }
    }
    if (cleanupFailures.length) failure = failure ?? new JourneyStepError(`Magic Auth E2E failed at ${[...new Set(cleanupFailures)].join(', ')}.`);
    await browser?.close();
    await pool.end();
  }
  if (failure) throw failure;
}

run().catch((error: Error) => {
  console.error(error.message);
  process.exitCode = 1;
});

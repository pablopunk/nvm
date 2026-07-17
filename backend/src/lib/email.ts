import { createDecipheriv, createHash, randomUUID } from 'node:crypto';
import { and, eq, inArray, sql } from 'drizzle-orm';
import { Webhook } from 'svix';
import { db } from '../db/client';
import { emailOutbox, emailSuppressions, invites, providerEvents } from '../db/schema';
import { env } from './env';
import { PRODUCTION_WEB_ORIGIN } from '../../../src/shared/public-origin';

function decryptInviteToken(value: string) { const [iv, tag, encrypted] = value.split('.'); const key = createHash('sha256').update(env('INVITE_INTENT_SECRET') || env('WORKOS_COOKIE_PASSWORD') || 'development-only-invite-secret').digest(); const decipher = createDecipheriv('aes-256-gcm', key, Buffer.from(iv, 'base64url')); decipher.setAuthTag(Buffer.from(tag, 'base64url')); return Buffer.concat([decipher.update(Buffer.from(encrypted, 'base64url')), decipher.final()]).toString(); }

export function inviteUrl(token: string) {
  return `${env('PUBLIC_SITE_URL') || PRODUCTION_WEB_ORIGIN}/invite#${token}`;
}

export async function leaseOutbox(limit = 10, owner = randomUUID()) {
  const expiry = new Date(Date.now() + 5 * 60 * 1000);
  return db.transaction(async (tx) => {
    const rows = (await tx.execute(sql`select id from email_outbox where (status = 'queued' or (status = 'sending' and lease_expires_at < now())) and available_at <= now() order by created_at asc limit ${limit} for update skip locked`)).rows as Array<{ id: string }>;
    const ids = rows.map((row) => String((row as { id: string }).id));
    if (!ids.length) return [];
    return tx.update(emailOutbox).set({ status: 'sending', leaseOwner: owner, leaseExpiresAt: expiry, attempts: sql`${emailOutbox.attempts} + 1` }).where(inArray(emailOutbox.id, ids)).returning();
  });
}

export async function sendOutboxBatch(limit = 10) {
  const key = env('RESEND_API_KEY');
  const from = env('INVITE_FROM_EMAIL');
  if (!key || !from) return { sent: 0, skipped: true, reason: 'provider_not_configured' };
  const rows = await leaseOutbox(limit);
  let sent = 0;
  for (const row of rows) {
    try {
      const [suppressed] = await db.select().from(emailSuppressions).where(eq(emailSuppressions.email, row.recipient)).limit(1);
      if (suppressed) { await db.update(emailOutbox).set({ status: 'cancelled', lastError: suppressed.reason, leaseOwner: null, leaseExpiresAt: null }).where(eq(emailOutbox.id, row.id)); continue; }
      const invite = (await db.select().from(invites).where(eq(invites.id, row.inviteId)).limit(1))[0];
      if (!invite) { await failOutbox(row, 'invite_missing'); continue; }
      const token = decryptInviteToken(row.tokenCiphertext);
      const response = await fetch('https://api.resend.com/emails', { method: 'POST', headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json', 'Idempotency-Key': row.idempotencyKey }, body: JSON.stringify({ from, to: [row.recipient], subject: 'Your Nevermind invite', html: `<p>Welcome to Nevermind.</p><p><a href="${inviteUrl(token)}">Continue to Nevermind</a></p>` }) });
      const body = await response.json().catch(() => ({})) as { id?: string; message?: string };
      if (response.ok) { await db.update(emailOutbox).set({ status: 'sent', providerMessageId: body.id, leaseOwner: null, leaseExpiresAt: null }).where(eq(emailOutbox.id, row.id)); await db.update(invites).set({ status: 'sent', sentAt: new Date() }).where(eq(invites.id, invite.id)); sent++; }
      else await failOutbox(row, body.message || `provider_${response.status}`);
    } catch (error) { await failOutbox(row, error instanceof Error ? error.message : 'transport_failed'); }
  }
  return { sent, skipped: false };
}

async function failOutbox(row: typeof emailOutbox.$inferSelect, error: string) {
  await db.update(emailOutbox).set({ status: row.attempts >= 5 ? 'failed' : 'queued', lastError: error, availableAt: new Date(Date.now() + Math.min(Math.max(row.attempts, 1) * 60000, 3600000)), leaseOwner: null, leaseExpiresAt: null }).where(eq(emailOutbox.id, row.id));
}

export function verifyProviderWebhook(raw: string, headers: { get(name: string): string | null }, now = Date.now()) {
  const secret = env('RESEND_WEBHOOK_SECRET');
  const svixId = headers.get('svix-id');
  const timestamp = headers.get('svix-timestamp');
  const signatures = headers.get('svix-signature');
  if (!secret || !svixId || !timestamp || !signatures || !/^\d+$/.test(timestamp)) return false;
  const timestampMs = Number(timestamp) * 1000;
  if (!Number.isSafeInteger(timestampMs) || Math.abs(now - timestampMs) > 5 * 60 * 1000) return false;
  try {
    new Webhook(secret).verify(raw, { 'svix-id': svixId, 'svix-timestamp': timestamp, 'svix-signature': signatures });
    return true;
  } catch { return false; }
}

export async function processProviderEvent(input: { id: string; type: string; emails?: string[]; messageId?: string; raw: string }) {
  const [event] = await db.insert(providerEvents).values({ provider: 'resend', eventId: input.id, eventType: input.type, payloadHash: createHash('sha256').update(input.raw).digest('hex'), processedAt: new Date() }).onConflictDoNothing().returning();
  if (!event) return { duplicate: true };
  let recipients = input.emails?.map((email) => email.toLowerCase()) ?? [];
  if (input.messageId) {
    const [outbox] = await db.select({ recipient: emailOutbox.recipient }).from(emailOutbox).where(eq(emailOutbox.providerMessageId, input.messageId)).limit(1);
    if (outbox && !recipients.includes(outbox.recipient)) recipients.push(outbox.recipient);
    await db.update(emailOutbox).set({ status: input.type === 'email.delivered' ? 'delivered' : input.type === 'email.bounced' ? 'bounced' : input.type === 'email.complained' ? 'complained' : 'sent' }).where(eq(emailOutbox.providerMessageId, input.messageId));
  }
  if (['email.bounced', 'email.complained'].includes(input.type)) for (const email of recipients) {
    await db.insert(emailSuppressions).values({ email, reason: input.type }).onConflictDoNothing();
    await db.update(emailOutbox).set({ status: 'cancelled', lastError: input.type, leaseOwner: null, leaseExpiresAt: null }).where(and(eq(emailOutbox.recipient, email), inArray(emailOutbox.status, ['queued', 'sending'])));
  }
  return { duplicate: false };
}

export function normalizeProviderRecipients(to: unknown, mappedRecipient?: string): string[] {
  const recipients = Array.isArray(to) ? to.filter((value): value is string => typeof value === 'string') : [];
  if (mappedRecipient && !recipients.includes(mappedRecipient)) recipients.push(mappedRecipient);
  return [...new Set(recipients.map((email) => email.trim().toLowerCase()).filter(Boolean))];
}

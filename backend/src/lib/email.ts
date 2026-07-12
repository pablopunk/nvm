import { createDecipheriv, createHash, randomUUID } from 'node:crypto';
import { eq, inArray, sql } from 'drizzle-orm';
import { db } from '../db/client';
import { emailOutbox, emailSuppressions, invites, providerEvents } from '../db/schema';
import { env } from './env';

function decryptInviteToken(value: string) { const [iv, tag, encrypted] = value.split('.'); const key = createHash('sha256').update(env('INVITE_INTENT_SECRET') || env('WORKOS_COOKIE_PASSWORD') || 'development-only-invite-secret').digest(); const decipher = createDecipheriv('aes-256-gcm', key, Buffer.from(iv, 'base64url')); decipher.setAuthTag(Buffer.from(tag, 'base64url')); return Buffer.concat([decipher.update(Buffer.from(encrypted, 'base64url')), decipher.final()]).toString(); }

export async function leaseOutbox(limit = 10, owner = randomUUID()) {
  const expiry = new Date(Date.now() + 5 * 60 * 1000);
  return db.transaction(async (tx) => {
    const rows = (await tx.execute(sql`select id from email_outbox where status = 'queued' and available_at <= now() and (lease_expires_at is null or lease_expires_at < now()) order by created_at asc limit ${limit} for update skip locked`)).rows as Array<{ id: string }>;
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
    const [suppressed] = await db.select().from(emailSuppressions).where(eq(emailSuppressions.email, row.recipient)).limit(1);
    if (suppressed) { await db.update(emailOutbox).set({ status: 'cancelled', lastError: suppressed.reason, leaseOwner: null, leaseExpiresAt: null }).where(eq(emailOutbox.id, row.id)); continue; }
    const invite = (await db.select().from(invites).where(eq(invites.id, row.inviteId)).limit(1))[0];
    if (!invite) continue;
    let token: string; try { token = decryptInviteToken(row.tokenCiphertext); } catch { await db.update(emailOutbox).set({ status: 'failed', lastError: 'token_decryption_failed', leaseOwner: null, leaseExpiresAt: null }).where(eq(emailOutbox.id, row.id)); continue; }
    const response = await fetch('https://api.resend.com/emails', { method: 'POST', headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json', 'Idempotency-Key': row.idempotencyKey }, body: JSON.stringify({ from, to: [row.recipient], subject: 'Your Nevermind invite', html: `<p>Welcome to Nevermind.</p><p><a href="${env('PUBLIC_SITE_URL') || 'https://nvm.fyi'}/invite#${token}">Continue to Nevermind</a></p>` }) });
    const body = await response.json().catch(() => ({})) as { id?: string; message?: string };
    if (response.ok) { await db.update(emailOutbox).set({ status: 'sent', providerMessageId: body.id, leaseOwner: null, leaseExpiresAt: null }).where(eq(emailOutbox.id, row.id)); await db.update(invites).set({ status: 'sent', sentAt: new Date() }).where(eq(invites.id, invite.id)); sent++; }
    else await db.update(emailOutbox).set({ status: row.attempts >= 5 ? 'failed' : 'queued', lastError: body.message || `provider_${response.status}`, availableAt: new Date(Date.now() + Math.min(row.attempts * 60000, 3600000)), leaseOwner: null, leaseExpiresAt: null }).where(eq(emailOutbox.id, row.id));
  }
  return { sent, skipped: false };
}

export function verifyProviderWebhook(raw: string, signature: string | null) {
  const secret = env('RESEND_WEBHOOK_SECRET');
  if (!secret || !signature) return false;
  return createHash('sha256').update(`${secret}.${raw}`).digest('hex') === signature;
}

export async function processProviderEvent(input: { id: string; type: string; email?: string; messageId?: string; raw: string }) {
  const [event] = await db.insert(providerEvents).values({ provider: 'resend', eventId: input.id, eventType: input.type, payloadHash: createHash('sha256').update(input.raw).digest('hex'), processedAt: new Date() }).onConflictDoNothing().returning();
  if (!event) return { duplicate: true };
  if (input.email && ['email.bounced', 'email.complained'].includes(input.type)) await db.insert(emailSuppressions).values({ email: input.email.toLowerCase(), reason: input.type }).onConflictDoNothing();
  if (input.messageId) await db.update(emailOutbox).set({ status: input.type === 'email.delivered' ? 'delivered' : input.type === 'email.bounced' ? 'bounced' : input.type === 'email.complained' ? 'complained' : 'sent' }).where(eq(emailOutbox.providerMessageId, input.messageId));
  return { duplicate: false };
}

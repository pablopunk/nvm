import { createHash, randomBytes } from 'node:crypto';
import { and, desc, eq, inArray } from 'drizzle-orm';
import { db } from '../db/client';
import { emailOutbox, invites, waitlistEntries } from '../db/schema';
import { env } from './env';

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
export const canonicalEmail = (value: string) => value.trim().toLowerCase();
export const isWaitlistEnabled = () => env('PUBLIC_WAITLIST_ENABLED') === 'true';
export const isInviteGateEnabled = () => env('INVITE_GATE_ENABLED') === 'true';

export function validateWaitlistEmail(value: unknown): string | null {
  if (typeof value !== 'string') return null;
  const email = canonicalEmail(value);
  return email.length <= 320 && emailPattern.test(email) ? email : null;
}

export async function captureWaitlist(email: string, ip?: string) {
  const ipHash = ip ? createHash('sha256').update(ip).digest('hex') : undefined;
  const [entry] = await db.insert(waitlistEntries).values({ email, ipHash }).onConflictDoNothing({ target: waitlistEntries.email }).returning();
  return entry ?? (await db.select().from(waitlistEntries).where(eq(waitlistEntries.email, email)).limit(1))[0];
}

export async function listWaitlist(status?: string) {
  return db.select().from(waitlistEntries).where(status ? eq(waitlistEntries.status, status) : undefined).orderBy(desc(waitlistEntries.submittedAt));
}

export async function createInvite(input: { email: string; waitlistEntryId?: string; createdBy?: string }) {
  const email = canonicalEmail(input.email);
  const existing = await db.select().from(invites).where(and(eq(invites.email, email), inArray(invites.status, ['queued', 'sending', 'sent']))).limit(1);
  if (existing[0]) return { invite: existing[0], token: null, existing: true };
  const token = randomBytes(32).toString('base64url');
  const days = Number(env('INVITE_TTL_DAYS') || 7);
  const expiresAt = new Date(Date.now() + (Number.isFinite(days) ? days : 7) * 86400000);
  return db.transaction(async (tx) => {
    const [invite] = await tx.insert(invites).values({ email, waitlistEntryId: input.waitlistEntryId, createdBy: input.createdBy, tokenHash: createHash('sha256').update(token).digest('hex'), expiresAt }).returning();
    await tx.insert(emailOutbox).values({ inviteId: invite.id, recipient: email, idempotencyKey: `invite/${invite.id}/v1` });
    if (input.waitlistEntryId) await tx.update(waitlistEntries).set({ status: 'invited', reviewedAt: new Date(), reviewerId: input.createdBy }).where(eq(waitlistEntries.id, input.waitlistEntryId));
    return { invite, token, existing: false };
  });
}

export async function redeemInvite(token: string, email: string) {
  const hash = createHash('sha256').update(token).digest('hex');
  return db.transaction(async (tx) => {
    const [invite] = await tx.select().from(invites).where(eq(invites.tokenHash, hash)).limit(1);
    if (!invite || invite.email !== canonicalEmail(email) || invite.expiresAt <= new Date() || !['queued', 'sending', 'sent'].includes(invite.status)) return null;
    const [redeemed] = await tx.update(invites).set({ status: 'redeemed', redeemedAt: new Date() }).where(and(eq(invites.id, invite.id), inArray(invites.status, ['queued', 'sending', 'sent']))).returning();
    if (!redeemed) return null;
    if (invite.waitlistEntryId) await tx.update(waitlistEntries).set({ status: 'redeemed' }).where(eq(waitlistEntries.id, invite.waitlistEntryId));
    return redeemed;
  });
}

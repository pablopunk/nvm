import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { createInvite, listWaitlist } from '../../../lib/waitlist';
import { db } from '../../../db/client';
import { waitlistEntries } from '../../../db/schema';
import { eq, inArray } from 'drizzle-orm';
import { env } from '../../../lib/env';

export const GET: APIRoute = async ({ request, url }) => {
  if (!await requireAdmin(request)) return new Response('Forbidden', { status: 403 });
  return Response.json({ entries: await listWaitlist(url.searchParams.get('status') ?? undefined) });
};

export const POST: APIRoute = async ({ request }) => {
  const admin = await requireAdmin(request);
  if (!admin) return new Response('Forbidden', { status: 403 });
  let body: { email?: string; waitlistEntryId?: string; entryIds?: string[] };
  try { body = await request.json(); } catch { return new Response('Invalid request', { status: 400 }); }
  if (body.entryIds) {
    const cap = Number(env('INVITE_BULK_CAP') || 50);
    if (body.entryIds.length === 0 || body.entryIds.length > cap) return new Response(`Bulk operations are limited to ${cap} entries`, { status: 400 });
    const entries = await db.select().from(waitlistEntries).where(inArray(waitlistEntries.id, body.entryIds));
    const results = await Promise.all(entries.filter((entry) => entry.status === 'pending').map((entry) => createInvite({ email: entry.email, waitlistEntryId: entry.id, createdBy: admin.id })));
    return Response.json({ created: results.filter((item) => !item.existing).length, skipped: entries.length - results.filter((item) => !item.existing).length });
  }
  if (!body.email) return new Response('Email required', { status: 400 });
  const result = await createInvite({ email: body.email, waitlistEntryId: body.waitlistEntryId, createdBy: admin.id });
  return Response.json({ invite: result.invite, queued: !result.existing }, { status: result.existing ? 200 : 201 });
};

export const PUT: APIRoute = async ({ request }) => {
  const admin = await requireAdmin(request);
  if (!admin) return new Response('Forbidden', { status: 403 });
  let body: { id?: string; status?: string; note?: string };
  try { body = await request.json(); } catch { return new Response('Invalid request', { status: 400 }); }
  if (!body.id || !['pending', 'declined', 'blocked', 'suppressed'].includes(body.status ?? '')) return new Response('Invalid status update', { status: 400 });
  const [entry] = await db.update(waitlistEntries).set({ status: body.status, reviewNote: body.note, reviewedAt: new Date(), reviewerId: admin.id }).where(eq(waitlistEntries.id, body.id)).returning();
  return entry ? Response.json({ entry }) : new Response('Not found', { status: 404 });
};

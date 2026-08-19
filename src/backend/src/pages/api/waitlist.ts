import type { APIRoute } from 'astro';
import { clientIp, rateLimitIp, tooManyRequests } from '../../lib/ratelimit';
import { captureWaitlist, isWaitlistEnabled, validateWaitlistEmail } from '../../lib/waitlist';

export const POST: APIRoute = async ({ request }) => {
  if (!isWaitlistEnabled()) return new Response(JSON.stringify({ message: 'Waitlist is not currently open.' }), { status: 404, headers: { 'Content-Type': 'application/json' } });
  const decision = await rateLimitIp('waitlist', clientIp(request), 5, '1 h');
  if (!decision.ok) return tooManyRequests(decision);
  let body: { email?: unknown; website?: unknown };
  try { body = await request.json(); } catch { return new Response('Invalid request', { status: 400 }); }
  const email = validateWaitlistEmail(body.email);
  if (!email) return new Response(JSON.stringify({ message: 'If eligible, we will email you.' }), { status: 202, headers: { 'Content-Type': 'application/json' } });
  if (body.website) return new Response(JSON.stringify({ message: 'If eligible, we will email you.' }), { status: 202, headers: { 'Content-Type': 'application/json' } });
  await captureWaitlist(email, clientIp(request) ?? undefined);
  return new Response(JSON.stringify({ message: 'If eligible, we will email you.' }), { status: 202, headers: { 'Content-Type': 'application/json' } });
};

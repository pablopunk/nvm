import type { APIRoute } from 'astro';
import { processProviderEvent, verifyProviderWebhook } from '../../../lib/email';

export const POST: APIRoute = async ({ request }) => {
  const raw = await request.text();
  if (!verifyProviderWebhook(raw, request.headers.get('x-resend-signature'))) return new Response('Invalid signature', { status: 401 });
  let body: { id?: string; type?: string; data?: { email?: string; email_id?: string } };
  try { body = JSON.parse(raw); } catch { return new Response('Invalid payload', { status: 400 }); }
  if (!body.id || !body.type) return new Response('Invalid event', { status: 400 });
  await processProviderEvent({ id: body.id, type: body.type, email: body.data?.email, messageId: body.data?.email_id, raw });
  return new Response(null, { status: 204 });
};

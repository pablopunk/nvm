import type { APIRoute } from 'astro';
import { createInviteIntent } from '../../../lib/waitlist';

export const POST: APIRoute = async ({ request, url }) => {
  let body: { token?: unknown };
  try { body = await request.json(); } catch { return new Response('Invalid request', { status: 400 }); }
  if (typeof body.token !== 'string' || body.token.length < 32) return new Response('Invite unavailable', { status: 404 });
  const result = await createInviteIntent(body.token);
  if (!result) return Response.redirect(new URL('/?invite=invalid', url), 303);
  const headers = new Headers({ Location: '/api/auth/signin?return_to=/invite/complete' });
  headers.append('Set-Cookie', result.cookie);
  return new Response(null, { status: 303, headers });
};

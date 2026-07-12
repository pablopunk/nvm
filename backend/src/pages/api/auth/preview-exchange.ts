import type { APIRoute } from 'astro';
import { consumePreviewSessionGrant } from '../../../lib/preview-auth';
import { SESSION_COOKIE } from '../../../lib/workos';

export const GET: APIRoute = async ({ url }) => {
  const grant = url.searchParams.get('grant');
  if (!grant) return new Response('Missing preview session grant', { status: 400 });

  const session = await consumePreviewSessionGrant(grant, url.origin);
  if (!session) return new Response('Invalid or expired preview session grant', { status: 400 });

  const headers = new Headers();
  headers.append(
    'Set-Cookie',
    `${SESSION_COOKIE}=${encodeURIComponent(session.sealedSession)}; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=${60 * 60 * 24 * 30}`,
  );
  headers.set('Location', session.returnTo);
  return new Response(null, { status: 302, headers });
};

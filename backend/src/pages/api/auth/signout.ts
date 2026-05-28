import type { APIRoute } from 'astro';
import { SESSION_COOKIE } from '../../../lib/workos';

export const POST: APIRoute = ({ url }) => {
  const isHttps = url.protocol === 'https:';
  const headers = new Headers();
  headers.append(
    'Set-Cookie',
    `${SESSION_COOKIE}=; Path=/; HttpOnly; ${isHttps ? 'Secure; ' : ''}SameSite=Lax; Max-Age=0`,
  );
  headers.set('Location', '/');
  return new Response(null, { status: 302, headers });
};

export const GET = POST;

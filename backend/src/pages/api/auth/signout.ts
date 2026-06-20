import type { APIRoute } from 'astro';
import { SESSION_COOKIE } from '../../../lib/workos';
import { requireSameOrigin } from '../../../lib/csrf';

export const POST: APIRoute = ({ request, url }) => {
  // Custom origin check: more granular than Astro's built-in checkOrigin —
  // compares against the actual request URL origin (not the site config),
  // which matters in multi-domain and preview-deploy setups.
  const originCheck = requireSameOrigin(request);
  if (originCheck) return originCheck;

  const isHttps = url.protocol === 'https:';
  const headers = new Headers();
  headers.append(
    'Set-Cookie',
    `${SESSION_COOKIE}=; Path=/; HttpOnly; ${isHttps ? 'Secure; ' : ''}SameSite=Lax; Max-Age=0`,
  );
  headers.set('Location', '/');
  return new Response(null, { status: 302, headers });
};

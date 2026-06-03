import type { APIRoute } from 'astro';
import { getTokenAndUserFromBearer, revokeApiToken } from '../../../lib/tokens';

export const DELETE: APIRoute = async ({ request }) => {
  const row = await getTokenAndUserFromBearer(request.headers.get('authorization'));
  if (!row) return new Response('Unauthorized', { status: 401 });
  await revokeApiToken(row.user.id, row.tokenId);
  return new Response(null, { status: 204 });
};

export const POST: APIRoute = async (ctx) => DELETE(ctx);

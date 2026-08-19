import type { APIRoute } from 'astro';
import { compatibilityHeaders, compatibilityManifestForRequest, requestIdFromHeaders } from '../../lib/compatibility';

export const GET: APIRoute = function getCompatibility({ request }) {
  const requestId = requestIdFromHeaders(request.headers);
  return Response.json(compatibilityManifestForRequest(request, { requestId, route: 'compatibility' }), {
    headers: compatibilityHeaders(requestId),
  });
};

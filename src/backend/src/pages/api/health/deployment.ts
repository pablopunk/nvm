import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { backendCommitSha, backendShortVersion } from '../../../lib/deployment-identity';

export const SIGNUP_POLICY_READER_CAPABILITY = 'signup-policy-reader-v1';

export const GET: APIRoute = async ({ request }) => {
  if (!(await requireAdmin(request))) return new Response('Forbidden', { status: 403 });
  return Response.json({
    ok: true,
    capabilities: [SIGNUP_POLICY_READER_CAPABILITY],
    version: backendShortVersion(),
    commit: backendCommitSha(),
  });
};

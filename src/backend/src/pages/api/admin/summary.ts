import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { loadAdminSummary, resolveSummaryMonth } from '../../../lib/admin-summary';

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  const month = resolveSummaryMonth(url.searchParams.get('month'));
  if (!month) {
    return Response.json(
      { error: { type: 'invalid_month', message: 'Use month=YYYY-MM.' } },
      { status: 400 },
    );
  }
  return Response.json(await loadAdminSummary(month));
};

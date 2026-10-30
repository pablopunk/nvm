import type { APIRoute } from 'astro';
import { requireAdmin } from '../../../lib/admin';
import { dashboardRead } from '../../../lib/dashboard-read';
import { loadAdminSummary, resolveSummaryMonth, type AdminSummarySection } from '../../../lib/admin-summary';

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
  const section = url.searchParams.get('section');
  if (section && !['overview', 'users', 'models', 'pricing', 'performance'].includes(section)) {
    return Response.json({ error: { type: 'invalid_section', message: 'Invalid summary section.' } }, { status: 400 });
  }
  return Response.json(await dashboardRead(() => loadAdminSummary(month, (section ?? undefined) as AdminSummarySection | undefined)));
};

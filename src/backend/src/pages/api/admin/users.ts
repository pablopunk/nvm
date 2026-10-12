import type { APIRoute } from 'astro';
import { sql } from 'drizzle-orm';
import { db } from '../../../db/client';
import { requireAdmin } from '../../../lib/admin';
import {
  decodeAdminPageCursor,
  encodeAdminPageCursor,
  parseAdminPageSize,
  reportBounds,
  resolveAdminReportRange,
  userCursorIdIsValid,
} from '../../../lib/admin-reporting';

function resultRows<T>(result: unknown): T[] {
  if (Array.isArray(result)) return result as T[];
  if (
    result &&
    typeof result === 'object' &&
    Array.isArray((result as { rows?: unknown }).rows)
  ) {
    return (result as { rows: T[] }).rows;
  }
  return [];
}

export const GET: APIRoute = async ({ request, url }) => {
  if (!(await requireAdmin(request)))
    return new Response('Forbidden', { status: 403 });
  const range = resolveAdminReportRange(
    url.searchParams.get('from'),
    url.searchParams.get('to'),
  );
  const pageSize = parseAdminPageSize(url.searchParams.get('limit'));
  const email = url.searchParams.get('email')?.trim().toLowerCase() ?? '';
  if (
    email.length > 320 ||
    (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email))
  ) {
    return Response.json(
      {
        error: {
          type: 'invalid_email_search',
          message: 'Provide a valid email address for account lookup.',
        },
      },
      { status: 400 },
    );
  }
  if (!range || pageSize == null) {
    return Response.json(
      {
        error: {
          type: 'invalid_report_range',
          message:
            'Provide a valid UTC range (maximum 12 months) and a page size from 1 to 100.',
        },
      },
      { status: 400 },
    );
  }
  const bounds = reportBounds(range);
  const cursor = decodeAdminPageCursor(
    url.searchParams.get('cursor'),
    bounds,
    userCursorIdIsValid,
  );
  if (cursor === undefined) {
    return Response.json(
      {
        error: {
          type: 'invalid_cursor',
          message:
            'The user cursor is invalid or belongs to another date range.',
        },
      },
      { status: 400 },
    );
  }

  const result = await db.execute<{
    id: string;
    email: string;
    plan: string;
    role: string;
    created_at: string;
    balance: number;
    free_balance: number;
    paid_balance: number;
    requests: number;
    spent: number;
    last_model: string | null;
    last_used_at: string | null;
  }>(sql`
    select
      u.id,
      u.email,
      u.plan,
      u.role,
      to_char(u.created_at at time zone 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') as created_at,
      coalesce((select sum(delta) from credit_ledger where user_id = u.id), 0)::int as balance,
      coalesce((select sum(delta) from credit_ledger where user_id = u.id and kind = 'free'), 0)::int as free_balance,
      coalesce((select sum(delta) from credit_ledger where user_id = u.id and kind = 'paid'), 0)::int as paid_balance,
      coalesce((select count(*) from usage where user_id = u.id and created_at >= ${range.from} and created_at < ${range.to}), 0)::int as requests,
      coalesce((select sum(cost_credits) from usage where user_id = u.id and created_at >= ${range.from} and created_at < ${range.to}), 0)::int as spent,
      (select model from usage where user_id = u.id and created_at >= ${range.from} and created_at < ${range.to} order by created_at desc limit 1) as last_model,
      (select created_at from usage where user_id = u.id and created_at >= ${range.from} and created_at < ${range.to} order by created_at desc limit 1) as last_used_at
    from users u
     where (${email || null}::text is not null or (u.created_at >= ${range.from} and u.created_at < ${range.to}))
      and (${email || null}::text is null or u.email = ${email || null})
      and (${cursor?.createdAt ?? null}::timestamptz is null
        or (u.created_at, u.id) < (${cursor?.createdAt ?? null}::timestamptz, ${cursor?.id ?? null}::uuid))
    order by u.created_at desc, u.id desc
    limit ${pageSize + 1}
  `);
  const rows = resultRows<{
    id: string;
    email: string;
    plan: string;
    role: string;
    created_at: string;
    balance: number;
    free_balance: number;
    paid_balance: number;
    requests: number;
    spent: number;
    last_model: string | null;
    last_used_at: string | null;
  }>(result);
  const hasMore = rows.length > pageSize;
  const page = hasMore ? rows.slice(0, pageSize) : rows;
  const last = page.at(-1);

  return Response.json({
    users: page.map((r) => ({
      id: r.id,
      email: r.email,
      plan: r.plan,
      role: r.role,
      createdAt: r.created_at,
      balance: r.balance,
      freeBalance: r.free_balance,
      paidBalance: r.paid_balance,
      requests: r.requests,
      spent: r.spent,
      lastModel: r.last_model,
      lastUsedAt: r.last_used_at,
    })),
    range: bounds,
    pageSize,
    ...(email
      ? { searchedEmail: email, searchScope: 'exact_email_all_history' }
      : {}),
    nextCursor:
      hasMore && last
        ? encodeAdminPageCursor({
            createdAt: last.created_at,
            id: last.id,
            bounds,
          })
        : null,
  });
};

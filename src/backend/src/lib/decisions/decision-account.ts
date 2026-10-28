import { PRODUCTION_WEB_ORIGIN } from '../../../../app/shared/public-origin';
import type { CreditKind } from '../credit-reservations';
import { decideProxyModel } from '../proxy';
import {
  extractPatFromHeaders,
  getUserFromHeaders,
  type PatHeaderName,
} from '../tokens';
import { ensureMonthlyFreeCredits, getBalances } from '../users';

const DASHBOARD_URL = `${PRODUCTION_WEB_ORIGIN}/dashboard`;

export type DecisionAccount = {
  user: { id: string; plan: string };
  kind: CreditKind;
};

export async function resolveDecisionAccount(
  request: Request,
  headerName: PatHeaderName,
): Promise<Response | DecisionAccount> {
  if (!extractPatFromHeaders(request, headerName))
    return new Response('Unauthorized', { status: 401 });
  const user = await getUserFromHeaders(request, headerName);
  if (!user) return new Response('Unauthorized', { status: 401 });
  await ensureMonthlyFreeCredits(user.id);
  const balances = await getBalances(user.id);
  if (balances.total <= 0)
    return Response.json(
      {
        error: {
          type: 'insufficient_credits',
          message: 'No credits remaining',
          dashboard_url: DASHBOARD_URL,
        },
      },
      { status: 402 },
    );
  const decision = decideProxyModel({
    plan: user.plan,
    paidCredits: balances.paid,
    freeCredits: balances.free,
    role: 'smart',
    headers: request.headers,
  });
  return { user: { id: user.id, plan: user.plan }, kind: decision.creditKind };
}

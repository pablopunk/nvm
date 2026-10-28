import type { APIRoute } from 'astro';
import { randomUUID } from 'node:crypto';
import {
  ModelNotConfiguredError,
  getDecisionModelRoute,
} from '../../../lib/settings';
import {
  backendKillSwitchEnabled,
  killSwitchResponse,
} from '../../../lib/compatibility';
import { resolveDecisionAccount } from '../../../lib/decisions/decision-account';
import { decisionErrorResponse } from '../../../lib/decisions/decision-errors';
import { runDecision } from '../../../lib/decisions/decision-execution';
import { parseDecisionRequest } from '../../../lib/decisions/decision-request';
import { MAX_PROXY_REQUEST_BODY_BYTES } from '../../../lib/limits';
import {
  readRequestBodyWithinLimit,
  requestTooLargeResponse,
  withRequestId,
} from '../../../lib/proxy';
import { rateLimitChat, tooManyRequests } from '../../../lib/ratelimit';

export const config = { maxDuration: 120 };

async function parseJsonBody(request: Request, requestId: string) {
  const bytes = await readRequestBodyWithinLimit(request);
  if (!bytes) return { response: requestTooLargeResponse(requestId) };
  try {
    return { json: JSON.parse(new TextDecoder().decode(bytes)) as unknown };
  } catch {
    return {
      response: decisionErrorResponse(
        400,
        'invalid_decision_request',
        'Decision requests must be JSON.',
      ),
    };
  }
}

async function currentDecisionRoute() {
  try {
    return await getDecisionModelRoute();
  } catch (error) {
    if (error instanceof ModelNotConfiguredError) return null;
    throw error;
  }
}

async function handleDecision(request: Request, requestId: string) {
  const body = await parseJsonBody(request, requestId);
  if ('response' in body) return body.response;

  const parsed = parseDecisionRequest(body.json);
  if (!parsed.ok)
    return decisionErrorResponse(
      400,
      'invalid_decision_request',
      parsed.message,
    );

  const account = await resolveDecisionAccount(request, 'authorization');
  if (account instanceof Response) return account;

  const rateDecision = await rateLimitChat(account.user.id, account.kind);
  if (!rateDecision.ok) return tooManyRequests(rateDecision);

  const route = await currentDecisionRoute();
  if (!route)
    return decisionErrorResponse(
      503,
      'decision_model_unavailable',
      'No decision model is configured.',
    );

  return runDecision({ requestId, account, route, request: parsed.request });
}

export const POST: APIRoute = async ({ request }) => {
  const requestId = randomUUID();
  if (backendKillSwitchEnabled('ai_proxy'))
    return killSwitchResponse(
      'ai_proxy',
      'AI decisions are temporarily disabled.',
      requestId,
    );
  const declaredBytes = Number(request.headers.get('content-length'));
  if (
    Number.isFinite(declaredBytes) &&
    declaredBytes > MAX_PROXY_REQUEST_BODY_BYTES
  )
    return requestTooLargeResponse(requestId);
  return withRequestId(await handleDecision(request, requestId), requestId);
};

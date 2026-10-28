import { randomUUID } from 'node:crypto';
import type { ExtensionDecisionResult } from '../../resources/nevermind-extension-api';
import { nevermindDesktopHeaders } from '../backend/nevermind-api';
import {
  getNevermindAuth,
  NevermindAuthRequiredError,
} from '../backend/nevermind-auth';
import {
  decisionRequestFailure,
  type DecisionWireRequest,
  parseDecisionWireResponse,
} from './extension-ai-decision-request';

const TRAILING_SLASH_PATTERN = /\/$/;

export async function requestNevermindDecision(
  body: DecisionWireRequest,
  signal?: AbortSignal,
): Promise<ExtensionDecisionResult> {
  const auth = await getNevermindAuth();
  if (!auth) throw new NevermindAuthRequiredError();
  const response = await fetch(
    `${auth.baseUrl.replace(TRAILING_SLASH_PATTERN, '')}/api/v1/decisions`,
    {
      method: 'POST',
      headers: nevermindDesktopHeaders({
        Authorization: `Bearer ${auth.token}`,
        'Content-Type': 'application/json',
        'X-Request-ID': randomUUID(),
      }),
      body: JSON.stringify(body),
      signal,
    },
  );
  const json = await response.json().catch(() => null);
  if (!response.ok) throw decisionRequestFailure(response.status, json);
  return parseDecisionWireResponse(json);
}

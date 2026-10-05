import {
  backendDiagnosticHeaders,
  isDiagnosticBackend,
  responseRequestId,
} from '../shared/backend-diagnostic-policy';
import {
  bindOperation,
  currentOperation,
  operationStage,
  recordServerOutcome,
  runOperation,
} from './observability';
import type { SimpleStreamOptions } from '@earendil-works/pi-ai/compat';
type ProviderResponseCallback = NonNullable<SimpleStreamOptions['onResponse']>;

export function observeProviderResponse(
  response: { status: number; headers: Record<string, string> },
  model: { baseUrl: string },
) {
  if (typeof model?.baseUrl !== 'string') return;
  if (!isDiagnosticBackend(`${model.baseUrl.replace(/\/$/, '')}/request`))
    return;
  recordServerOutcome(response.status, responseRequestId(response.headers));
}

export function observeProviderOptions(options: SimpleStreamOptions = {}) {
  const original = options.onResponse;
  return {
    ...options,
    onResponse: bindOperation(async function providerResponded(
      response: Parameters<ProviderResponseCallback>[0],
      model: Parameters<ProviderResponseCallback>[1],
    ) {
      observeProviderResponse(response, model);
      return original?.(response, model);
    }),
  };
}

export function observeAgentResponses(agent: {
  streamFn?: import('@earendil-works/pi-coding-agent').AgentSession['agent']['streamFn'];
}) {
  const original = agent.streamFn;
  if (!original) return;
  agent.streamFn = function observedAgentStream(model, context, options) {
    return original.call(
      agent,
      model,
      context,
      observeProviderOptions(options),
    );
  };
}

export function fetchDiagnosticBackend(url: string, init?: RequestInit) {
  return runOperation('backend.request', async function fetchBackendResponse() {
    operationStage('request');
    const headers = new Headers(init?.headers);
    for (const [name, value] of Object.entries(
      backendDiagnosticHeaders(url, currentOperation()),
    ))
      headers.set(name, value);
    const response = await fetch(url, {
      ...init,
      headers,
      ...(isDiagnosticBackend(url) ? { redirect: 'error' as const } : {}),
    });
    if (isDiagnosticBackend(url))
      recordServerOutcome(response.status, responseRequestId(response.headers));
    return response;
  });
}

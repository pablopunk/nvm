import type { ParsedDecisionResponse } from './decision-contract';
import type { DecisionRequest } from './decision-request';
import {
  buildOpenRouterDecisionBody,
  openRouterDecisionsUrl,
  parseOpenRouterDecisionResponse,
} from './decision-openrouter';

export type DecisionProvider = 'openrouter';

export type DecisionAdapter = {
  provider: DecisionProvider;
  endpointUrl: (upstreamBaseUrl: string) => string;
  headers: (apiKey: string) => Record<string, string>;
  buildBody: (request: DecisionRequest, modelId: string) => unknown;
  parseResponse: (
    body: unknown,
    request: DecisionRequest,
  ) => ParsedDecisionResponse;
};

const openRouterAdapter: DecisionAdapter = {
  provider: 'openrouter',
  endpointUrl: openRouterDecisionsUrl,
  headers: (apiKey) => ({
    authorization: `Bearer ${apiKey}`,
    'content-type': 'application/json',
  }),
  buildBody: buildOpenRouterDecisionBody,
  parseResponse: parseOpenRouterDecisionResponse,
};

const ADAPTERS: Record<DecisionProvider, DecisionAdapter> = {
  openrouter: openRouterAdapter,
};

export function decisionAdapterFor(provider: string): DecisionAdapter | null {
  return Object.hasOwn(ADAPTERS, provider)
    ? ADAPTERS[provider as DecisionProvider]
    : null;
}

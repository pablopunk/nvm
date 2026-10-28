import { computeMaximumTokenPricedCost, type ModelCost } from '../cost';
import { finalizeReservation, reserveCredits } from '../credit-reservations';
import { freeSpendCapErrorBody } from '../free-spend-cap';
import { log } from '../log';
import { getUpstreamConfig, UpstreamConfigError } from '../upstream';
import { imageDimensions } from '../../../../app/shared/ai-image-dimensions';
import type { DecisionAccount } from './decision-account';
import { decisionAdapterFor } from './decision-adapter';
import {
  decisionOutputTokenCeiling,
  decisionReservationCostUsd,
  decisionReservationCredits,
  settleDecisionCost,
} from './decision-billing';
import {
  DecisionProviderResponseError,
  type ParsedDecisionResponse,
} from './decision-contract';
import {
  decisionErrorResponse,
  decisionProviderStatusResponse,
} from './decision-errors';
import {
  findEligibleDecisionModel,
  lookupDecisionModelCost,
} from './decision-models';
import type { DecisionRequest } from './decision-request';

export const DECISION_PROVIDER_TIMEOUT_MS = 90_000;

export type DecisionRoute = { provider: string; modelId: string };

type Dependencies = {
  reserve: typeof reserveCredits;
  finalize: typeof finalizeReservation;
  lookupCost: typeof lookupDecisionModelCost;
  findModel: typeof findEligibleDecisionModel;
  upstream: typeof getUpstreamConfig;
  fetchImpl: (url: string, init: RequestInit) => Promise<Response>;
};

const defaultDependencies: Dependencies = {
  reserve: reserveCredits,
  finalize: finalizeReservation,
  lookupCost: lookupDecisionModelCost,
  findModel: findEligibleDecisionModel,
  upstream: getUpstreamConfig,
  fetchImpl: (input, init) => fetch(input, init),
};

function reservationFailureResponse(
  reason:
    | 'insufficient_credits'
    | 'request_already_reserved'
    | 'invalid_audio_preparation'
    | 'free_spend_cap_reached',
  balance: number,
  reserved: number,
  credits: number,
): Response {
  if (reason === 'free_spend_cap_reached')
    return Response.json(freeSpendCapErrorBody(), { status: 503 });
  if (reason === 'request_already_reserved')
    return decisionErrorResponse(
      409,
      'invalid_decision_request',
      'This request is already being processed.',
    );
  return Response.json(
    {
      error: {
        type: 'insufficient_credits',
        message: 'Request cost would exceed remaining balance',
        estimated_credits: credits,
        balance: balance - reserved,
      },
    },
    { status: 402 },
  );
}

async function releaseReservation(
  requestId: string,
  account: DecisionAccount,
  deps: Dependencies,
) {
  await deps.finalize({
    requestId,
    expectedUserId: account.user.id,
    outcome: 'release',
  });
}

async function settleSuccess(input: {
  requestId: string;
  account: DecisionAccount;
  route: DecisionRoute;
  costRow: ModelCost;
  reservedCostUsd: number;
  parsed?: ParsedDecisionResponse;
  startedAt: number;
  deps: Dependencies;
  dedup?: {
    userId: string;
    idempotencyKey: string;
    requestHash: string;
    responseJson: unknown;
  };
}) {
  const settlement = settleDecisionCost({
    costRow: input.costRow,
    reservedCostUsd: input.reservedCostUsd,
    usage: input.parsed?.usage,
    providerCostUsd: input.parsed?.providerCostUsd,
  });
  if (settlement.providerCostUsd > input.reservedCostUsd)
    log.warn('decision_cost_exceeded_reservation', {
      request_id: input.requestId,
      reserved_cost_usd: input.reservedCostUsd,
      settled_cost_usd: settlement.providerCostUsd,
      cost_source: settlement.costSource,
    });
  await input.deps.finalize({
    requestId: input.requestId,
    expectedUserId: input.account.user.id,
    outcome: 'settle',
    model: input.route.modelId,
    provider: input.route.provider,
    tokens: settlement.tokens,
    costRow: input.costRow,
    providerCostUsd: settlement.providerCostUsd,
    costSource: settlement.costSource,
    modality: 'text',
    status: 200,
    latencyMs: Date.now() - input.startedAt,
    ...(input.dedup
      ? {
          dedup: {
            ...input.dedup,
            status: 'completed' as const,
            upstreamStatus: 200,
          },
        }
      : {}),
  });
}

type ProviderInvocation =
  | { response: Response }
  | { unavailable: true; ambiguous: boolean };

/** A timed-out request may already have been billed upstream, so it is not a clean failure. */
function isTimeoutOrAbort(error: unknown): boolean {
  return (
    error instanceof DOMException &&
    (error.name === 'TimeoutError' || error.name === 'AbortError')
  );
}

async function invokeProvider(input: {
  route: DecisionRoute;
  request: DecisionRequest;
  signal: AbortSignal;
  deps: Dependencies;
}): Promise<ProviderInvocation> {
  const adapter = decisionAdapterFor(input.route.provider);
  if (!adapter) return { unavailable: true, ambiguous: false };
  let upstream: ReturnType<typeof getUpstreamConfig>;
  try {
    upstream = input.deps.upstream(input.route.provider);
  } catch (error) {
    if (error instanceof UpstreamConfigError)
      return { unavailable: true, ambiguous: false };
    throw error;
  }
  try {
    const response = await input.deps.fetchImpl(
      adapter.endpointUrl(upstream.baseUrl),
      {
        method: 'POST',
        headers: adapter.headers(upstream.apiKey),
        body: JSON.stringify(
          adapter.buildBody(input.request, input.route.modelId),
        ),
        signal: input.signal,
      },
    );
    return { response };
  } catch (error) {
    return { unavailable: true, ambiguous: isTimeoutOrAbort(error) };
  }
}

async function readJsonOrNull(response: Response): Promise<unknown | null> {
  return response.json().catch(() => null);
}

/**
 * Reserves from the account's pool, calls the configured decision model once,
 * and settles exactly once. Failed requests release the user's reservation;
 * provider costs may still be incurred when the outcome is unknown.
 */
export async function runDecision(input: {
  requestId: string;
  account: DecisionAccount;
  route: DecisionRoute;
  request: DecisionRequest;
  deps?: Partial<Dependencies>;
  dedup?: { userId: string; idempotencyKey: string; requestHash: string };
}): Promise<Response> {
  const deps: Dependencies = { ...defaultDependencies, ...input.deps };
  const startedAt = Date.now();
  const { requestId, account, route, request } = input;

  const adapter = decisionAdapterFor(route.provider);
  if (!adapter)
    return decisionErrorResponse(
      503,
      'decision_model_unavailable',
      'The decision model provider is not supported.',
    );
  let imageInputLimit: number | undefined;
  if (request.images.length > 0) {
    const model = await deps
      .findModel(route.provider, route.modelId)
      .catch(() => null);
    if (!model)
      return decisionErrorResponse(
        503,
        'decision_model_unavailable',
        'The decision model catalog is unavailable. Try again shortly.',
      );
    if (!model.acceptsImages)
      return decisionErrorResponse(
        400,
        'unsupported_image_route',
        `The configured decision model ${route.modelId} does not accept images. Remove the images or ask the admin to configure an image-capable decision model.`,
      );
    if (!(model.contextLength && model.imageTokenPriced))
      return decisionErrorResponse(
        503,
        'decision_model_unavailable',
        'Image billing limits are unavailable for the configured decision model.',
      );
    if (
      route.modelId.startsWith('typesafe/jev-') &&
      request.images.some((image) => image.mediaType === 'image/webp')
    )
      return decisionErrorResponse(
        400,
        'unsupported_image_route',
        'The configured decision model accepts PNG and JPEG images.',
      );
    if (route.modelId.startsWith('typesafe/jev-')) {
      const invalid = request.images.some((image) => {
        const dimensions = imageDimensions(
          Buffer.from(image.data, 'base64'),
          image.mediaType,
        );
        if (!dimensions) return true;
        const { width, height } = dimensions;
        return (
          width < 256 ||
          height < 256 ||
          width > 1568 ||
          height > 1568 ||
          width * height < 78_400 ||
          width * height > 2_458_624
        );
      });
      if (invalid)
        return decisionErrorResponse(
          400,
          'invalid_decision_request',
          'Images for this model must be valid PNG or JPEG files, at least 256 pixels per side and 78,400 pixels total, and at most 1,568 pixels per side.',
        );
    }
    imageInputLimit = model.contextLength;
  }

  const costRow = await deps.lookupCost(route.provider, route.modelId);
  if (!costRow)
    return decisionErrorResponse(
      503,
      'decision_model_unavailable',
      'The decision model has no verified pricing.',
    );
  if (
    costRow.outputUsdPerMtok !== 0 ||
    costRow.tiers?.some(
      (tier) => (tier.outputUsdPerMtok ?? costRow.outputUsdPerMtok) !== 0,
    )
  )
    return decisionErrorResponse(
      503,
      'decision_model_unavailable',
      'The configured decision model has no verified output cost bound.',
    );

  const reservedCostUsd =
    imageInputLimit === undefined
      ? decisionReservationCostUsd(costRow, request)
      : computeMaximumTokenPricedCost(
          costRow,
          imageInputLimit,
          decisionOutputTokenCeiling(request),
        );
  // Reserve the estimated user charge while the free-spend cap reserves the
  // full provider ceiling. Any provider overrun is absorbed by Nevermind.
  const credits = decisionReservationCredits(costRow, request);
  const reservation = await deps.reserve({
    requestId,
    userId: account.user.id,
    kind: account.kind,
    credits,
    estimatedProviderCostUsd: reservedCostUsd,
    estimateCostSource: 'catalog_estimate',
  });
  if (!reservation.ok)
    return reservationFailureResponse(
      reservation.reason,
      reservation.balance,
      reservation.reserved,
      credits,
    );

  const invocation = await invokeProvider({
    route,
    request,
    signal: AbortSignal.timeout(DECISION_PROVIDER_TIMEOUT_MS),
    deps,
  });
  if ('unavailable' in invocation) {
    await releaseReservation(requestId, account, deps);
    if (invocation.ambiguous)
      log.warn('decision_provider_cost_unknown', {
        request_id: requestId,
        provider: route.provider,
        model: route.modelId,
      });
    return decisionErrorResponse(
      502,
      'decision_provider_unavailable',
      'The decision model is unavailable. Try again shortly.',
    );
  }
  if (!invocation.response.ok) {
    await invocation.response.body?.cancel();
    await releaseReservation(requestId, account, deps);
    log.warn('decision_provider_failed', {
      request_id: requestId,
      provider: route.provider,
      model: route.modelId,
      upstream_status: invocation.response.status,
    });
    return decisionProviderStatusResponse(invocation.response.status);
  }

  const body = await readJsonOrNull(invocation.response);
  let parsed: ParsedDecisionResponse;
  try {
    parsed = adapter.parseResponse(body, request);
  } catch (error) {
    if (!(error instanceof DecisionProviderResponseError)) throw error;
    await releaseReservation(requestId, account, deps);
    log.error('decision_provider_response_invalid', {
      request_id: requestId,
      provider: route.provider,
      model: route.modelId,
      reason: error.message,
      provider_cost_unknown: true,
    });
    return decisionErrorResponse(
      502,
      'decision_provider_response_invalid',
      'The decision model returned an invalid answer. Try again.',
    );
  }

  const responseBody = {
    id: requestId,
    model: parsed.providerModel ?? route.modelId,
    answers: parsed.answers,
    usage: parsed.usage,
  };
  await settleSuccess({
    requestId,
    account,
    route,
    costRow,
    reservedCostUsd,
    parsed,
    startedAt,
    deps,
    ...(input.dedup
      ? { dedup: { ...input.dedup, responseJson: responseBody } }
      : {}),
  });
  log.info('decision_completed', {
    request_id: requestId,
    user_id: account.user.id,
    provider: route.provider,
    model: route.modelId,
    question_count: request.questions.length,
    image_count: request.images.length,
    latency_ms: Date.now() - startedAt,
  });
  return Response.json(responseBody);
}

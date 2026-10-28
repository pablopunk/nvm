import type { APIRoute } from 'astro';
import {
  joinPublicApiUrl,
  parsePublicOrigin,
} from '../../../../../app/shared/public-origin';
import { chooseFundedAccountRoute } from '../../../lib/active-model-funding';
import {
  compatibilityHeaders,
  requestIdFromHeaders,
} from '../../../lib/compatibility';
import { computeUsdCost, usdToMicrocents } from '../../../lib/cost';
import { getPendingCreditReservations } from '../../../lib/credit-reservations';
import { env } from '../../../lib/env';
import {
  canReserveFreeSpend,
  getCurrentFreeSpendBudget,
  microcentsForFreePool,
} from '../../../lib/free-spend-cap';
import { estimateRequestCredits, MAX_INPUT_TOKENS } from '../../../lib/limits';
import { localDevelopmentOrigin } from '../../../lib/local-development-origin';
import { previewTargetFromEnvironment } from '../../../lib/preview-auth';
import { lookupModelCost, lookupModelDescriptor } from '../../../lib/pricing';
import {
  decideAccountModel,
  getModelRoute,
  ModelNotConfiguredError,
  parseExtensionAiModelRole,
} from '../../../lib/settings';
import { getUserFromBearer } from '../../../lib/tokens';
import { selectApiForModel } from '../../../lib/upstream';
import { ensureMonthlyFreeCredits, getBalances } from '../../../lib/users';

const NEVERMIND_PROVIDER_ID = 'nevermind';

export const GET: APIRoute = async ({ request }) => {
  const requestOrigin = new URL(request.url).origin;
  const previewTarget =
    env('VERCEL_ENV') === 'preview' ? previewTargetFromEnvironment() : null;
  const localOrigin = localDevelopmentOrigin(requestOrigin);
  if (
    env('VERCEL_ENV') === 'preview' &&
    (!previewTarget || previewTarget.origin !== requestOrigin)
  ) {
    return Response.json(
      {
        error: {
          type: 'configuration_error',
          message: 'Preview API origin is unavailable.',
        },
      },
      { status: 503 },
    );
  }

  let configuredApiOrigin: string;
  try {
    configuredApiOrigin =
      previewTarget?.origin ??
      localOrigin ??
      parsePublicOrigin(
        env('PUBLIC_API_ORIGIN') ?? 'https://api.nvm.fyi',
        'production_api',
      );
  } catch {
    return Response.json(
      {
        error: {
          type: 'configuration_error',
          message: 'Public API origin is unavailable.',
        },
      },
      { status: 503 },
    );
  }
  const user = await getUserFromBearer(request.headers.get('authorization'));
  if (!user) return new Response('Unauthorized', { status: 401 });
  await ensureMonthlyFreeCredits(user.id);

  const balances = await getBalances(user.id);
  const url = new URL(request.url);
  const inputTokensQ = url.searchParams.get('inputTokens');
  const charsQ = url.searchParams.get('chars');
  const requestedTokenCount = inputTokensQ
    ? Number(inputTokensQ)
    : charsQ
      ? Math.max(0, Number(charsQ) || 0) / 4
      : 0;
  const tokens = Number.isFinite(requestedTokenCount)
    ? Math.ceil(Math.max(0, requestedTokenCount))
    : Number.POSITIVE_INFINITY;
  const requestedSizeInvalid =
    !Number.isSafeInteger(tokens) || tokens > MAX_INPUT_TOKENS;
  const pending =
    tokens > 0 && !requestedSizeInvalid
      ? await getPendingCreditReservations(user.id)
      : { free: 0, paid: 0 };
  const availableBalances = {
    free: balances.free - pending.free,
    paid: balances.paid - pending.paid,
    total: balances.total - pending.free - pending.paid,
  };
  const requestedModel = parseExtensionAiModelRole(
    request.headers.get('x-nevermind-ai-model') ||
      url.searchParams.get('model'),
  );
  const role = requestedModel ?? 'smart';
  const forceFree =
    request.headers.get('x-nevermind-ai-model-tier') === 'free' ||
    url.searchParams.get('tier') === 'free';
  let decision = decideAccountModel({
    plan: user.plan,
    paidCredits: balances.paid,
    freeCredits: balances.free,
    role,
    forceFree,
  });
  let { modelTier, creditKind } = {
    modelTier: decision.tier,
    creditKind: decision.creditKind,
  };
  let provider: string;
  let modelId: string;
  let thinkingLevel: string;
  let descriptor: Awaited<ReturnType<typeof lookupModelDescriptor>> = null;
  let costEstimate: number | undefined;
  let estimatedProviderCostUsd: number | undefined;
  try {
    let route = await getModelRoute(decision.routeSlot);
    descriptor = await lookupModelDescriptor(route.provider, route.modelId);
    if (tokens > 0 && !requestedSizeInvalid && descriptor) {
      const costRow = await lookupModelCost(route.provider, route.modelId);
      if (costRow) {
        const maxOutputTokens = descriptor.maxTokens ?? 32_000;
        costEstimate = estimateRequestCredits(tokens, maxOutputTokens, costRow);
        estimatedProviderCostUsd = computeUsdCost(
          costRow,
          tokens,
          maxOutputTokens,
        );
        if (decision.tier === 'pro' && costEstimate > availableBalances.total) {
          const freeDecision = decideAccountModel({
            plan: user.plan,
            paidCredits: balances.paid,
            freeCredits: balances.free,
            role,
            forceFree: true,
          });
          const freeRoute = await getModelRoute(freeDecision.routeSlot);
          const freeDescriptor = await lookupModelDescriptor(
            freeRoute.provider,
            freeRoute.modelId,
          );
          const freeCostRow = await lookupModelCost(
            freeRoute.provider,
            freeRoute.modelId,
          );
          const freeMaxOutputTokens = freeDescriptor?.maxTokens ?? 32_000;
          const freeEstimate =
            freeDescriptor && freeCostRow
              ? estimateRequestCredits(tokens, freeMaxOutputTokens, freeCostRow)
              : undefined;
          const freeProviderCostUsd =
            freeDescriptor && freeCostRow
              ? computeUsdCost(freeCostRow, tokens, freeMaxOutputTokens)
              : undefined;
          const fundingChoice = chooseFundedAccountRoute({
            tier: decision.tier,
            freeCredits: availableBalances.free,
            paidCredits: availableBalances.paid,
            routeCredits: costEstimate,
            freeRouteCredits: freeEstimate,
            freeFallbackAllowed:
              (await selectApiForModel(freeRoute.provider, freeRoute.modelId)) ===
              (await selectApiForModel(route.provider, route.modelId)),
          });
          if (fundingChoice.freeFallback) {
            decision = freeDecision;
            modelTier = decision.tier;
            creditKind = decision.creditKind;
            route = freeRoute;
            descriptor = freeDescriptor;
            costEstimate = freeEstimate;
            estimatedProviderCostUsd = freeProviderCostUsd;
          }
        }
      }
    }
    provider = route.provider;
    modelId = route.modelId;
    thinkingLevel = route.thinkingLevel;
  } catch (err) {
    if (err instanceof ModelNotConfiguredError) {
      return Response.json(
        {
          error: {
            type: 'model_not_configured',
            message: 'No active model configured.',
          },
        },
        { status: 503 },
      );
    }
    throw err;
  }

  descriptor ??= await lookupModelDescriptor(provider, modelId);
  if (!descriptor) {
    return Response.json(
      {
        error: {
          type: 'model_descriptor_unavailable',
          message: `No descriptor for ${provider}/${modelId}`,
        },
      },
      { status: 503 },
    );
  }

  const api = await selectApiForModel(provider, modelId);
  const requestScopedOrigin = previewTarget || localOrigin;
  const baseUrl =
    api === 'anthropic-messages'
      ? requestScopedOrigin
        ? `${configuredApiOrigin}/api`
        : joinPublicApiUrl(configuredApiOrigin, '/api')
      : requestScopedOrigin
        ? `${configuredApiOrigin}/api/v1`
        : joinPublicApiUrl(configuredApiOrigin, '/api/v1');

  let notice: 'ok' | 'low' | 'blocked';
  const freeSpendWouldBeNeeded =
    !requestedSizeInvalid &&
    availableBalances.free > 0 &&
    availableBalances.paid < (costEstimate ?? 1);
  const freeSpendBudget = freeSpendWouldBeNeeded
    ? await getCurrentFreeSpendBudget()
    : null;
  const totalCostMicrocents =
    estimatedProviderCostUsd === undefined
      ? undefined
      : usdToMicrocents(estimatedProviderCostUsd);
  const freeSpendRequestCostMicrocents =
    costEstimate !== undefined &&
    Number.isSafeInteger(costEstimate) &&
    totalCostMicrocents !== undefined &&
    Number.isSafeInteger(totalCostMicrocents) &&
    totalCostMicrocents >= 0
      ? microcentsForFreePool({
          totalCostMicrocents,
          freeCredits: Math.min(
            costEstimate,
            Math.max(0, availableBalances.free),
          ),
          totalCredits: costEstimate,
        })
      : undefined;
  const freeSpendCapExhausted =
    freeSpendBudget?.capMicrocents != null &&
    (freeSpendBudget.halted ||
      !freeSpendBudget.baselineComplete ||
      freeSpendBudget.capMicrocents -
        freeSpendBudget.actualFreeCostMicrocents -
        freeSpendBudget.reservedFreeCostMicrocents <=
        0);
  const freeSpendRequestDoesNotFit =
    freeSpendBudget?.capMicrocents != null &&
    freeSpendRequestCostMicrocents !== undefined &&
    !canReserveFreeSpend({
      budget: freeSpendBudget,
      requestedMicrocents: freeSpendRequestCostMicrocents,
    });
  const freeSpendBlocked =
    freeSpendWouldBeNeeded &&
    (freeSpendCapExhausted || freeSpendRequestDoesNotFit);
  const requestedEstimateUnavailable =
    requestedSizeInvalid || (tokens > 0 && costEstimate === undefined);
  if (
    requestedEstimateUnavailable ||
    availableBalances.free < 0 ||
    availableBalances.paid < 0 ||
    availableBalances.total < (costEstimate ?? 1)
  ) {
    notice = 'blocked';
  } else if (freeSpendBlocked) {
    notice = 'blocked';
  } else {
    notice = 'ok';
  }

  const requestId = requestIdFromHeaders(request.headers);
  return Response.json(
    {
      ...descriptor,
      maxInputTokens: MAX_INPUT_TOKENS,
      thinkingLevel,
      api,
      provider: NEVERMIND_PROVIDER_ID,
      baseUrl,
      modelTier,
      creditKind,
      credits: {
        paid: balances.paid,
        free: balances.free,
        total: balances.total,
      },
      notice,
      ...(freeSpendBlocked ? { noticeReason: 'free_spend_cap_reached' } : {}),
      ...(costEstimate !== undefined ? { costEstimate } : {}),
    },
    { headers: compatibilityHeaders(requestId) },
  );
};

export const OPTIONS: APIRoute = async () =>
  new Response(null, { status: 404 });

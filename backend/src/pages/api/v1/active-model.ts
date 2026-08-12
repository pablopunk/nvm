import type { APIRoute } from 'astro';
import { getUserFromBearer } from '../../../lib/tokens';
import { getModelRoute, modelRouteSlotForAccount, ModelNotConfiguredError, parseExtensionAiModelRole } from '../../../lib/settings';
import { ensureMonthlyFreeCredits, getBalances } from '../../../lib/users';
import { lookupModelCost, lookupModelDescriptor } from '../../../lib/pricing';
import { compatibilityHeaders, requestIdFromHeaders } from '../../../lib/compatibility';
import { selectApiForModel } from '../../../lib/upstream';
import { estimateRequestCredits } from '../../../lib/limits';
import { env } from '../../../lib/env';
import { joinPublicApiUrl, parsePublicOrigin } from '../../../../../src/shared/public-origin';
import { previewTargetFromEnvironment } from '../../../lib/preview-auth';

const NEVERMIND_PROVIDER_ID = 'nevermind';

export const GET: APIRoute = async ({ request }) => {
  const requestOrigin = new URL(request.url).origin;
  const previewTarget = env('VERCEL_ENV') === 'preview' ? previewTargetFromEnvironment() : null;
  if (env('VERCEL_ENV') === 'preview' && (!previewTarget || previewTarget.origin !== requestOrigin)) {
    return Response.json({ error: { type: 'configuration_error', message: 'Preview API origin is unavailable.' } }, { status: 503 });
  }

  let configuredApiOrigin: string;
  try {
    configuredApiOrigin = previewTarget?.origin ?? parsePublicOrigin(env('PUBLIC_API_ORIGIN') ?? 'https://api.nvm.fyi', 'production_api');
  } catch {
    return Response.json({ error: { type: 'configuration_error', message: 'Public API origin is unavailable.' } }, { status: 503 });
  }
  const user = await getUserFromBearer(request.headers.get('authorization'));
  if (!user) return new Response('Unauthorized', { status: 401 });
  await ensureMonthlyFreeCredits(user.id);

  const balances = await getBalances(user.id);
  const url = new URL(request.url);
  const requestedModel = parseExtensionAiModelRole(request.headers.get('x-nevermind-ai-model') || url.searchParams.get('model'));
  const role = requestedModel ?? 'smart';
  const forceFree = request.headers.get('x-nevermind-ai-model-tier') === 'free' || url.searchParams.get('tier') === 'free';
  let modelTier = modelRouteSlotForAccount(user.plan, balances.paid, role, forceFree).startsWith('pro-') ? 'pro' : 'free';
  let creditKind: 'paid' | 'free' = forceFree || balances.paid <= 0 ? 'free' : 'paid';
  let provider: string;
  let modelId: string;
  let thinkingLevel: string;
  let descriptor: Awaited<ReturnType<typeof lookupModelDescriptor>> = null;
  let costEstimate: number | undefined;
  try {
    let route = await getModelRoute(modelRouteSlotForAccount(user.plan, balances.paid, role, forceFree));
    descriptor = await lookupModelDescriptor(route.provider, route.modelId);
    const inputTokensQ = url.searchParams.get('inputTokens');
    const charsQ = url.searchParams.get('chars');
    const tokens = inputTokensQ
      ? Math.max(0, Number(inputTokensQ) || 0)
      : charsQ ? Math.ceil(Math.max(0, Number(charsQ) || 0) / 4) : 0;
    if (tokens > 0 && descriptor) {
      const costRow = await lookupModelCost(route.provider, route.modelId);
      if (costRow) {
        costEstimate = estimateRequestCredits(tokens, descriptor.maxTokens ?? 32_000, costRow);
        if (creditKind === 'paid' && costEstimate > balances.paid) {
          creditKind = 'free';
          if (modelTier === 'pro') {
            modelTier = 'free';
            route = await getModelRoute(modelRouteSlotForAccount(user.plan, balances.paid, role, true));
            descriptor = await lookupModelDescriptor(route.provider, route.modelId);
            const freeCostRow = await lookupModelCost(route.provider, route.modelId);
            if (descriptor && freeCostRow) {
              costEstimate = estimateRequestCredits(tokens, descriptor.maxTokens ?? 32_000, freeCostRow);
            }
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
        { error: { type: 'model_not_configured', message: 'No active model configured.' } },
        { status: 503 },
      );
    }
    throw err;
  }

  descriptor ??= await lookupModelDescriptor(provider, modelId);
  if (!descriptor) {
    return Response.json(
      { error: { type: 'model_descriptor_unavailable', message: `No descriptor for ${provider}/${modelId}` } },
      { status: 503 },
    );
  }

  const api = selectApiForModel(provider, modelId);
  const baseUrl = api === 'anthropic-messages'
    ? previewTarget ? `${configuredApiOrigin}/api` : joinPublicApiUrl(configuredApiOrigin, '/api')
    : previewTarget ? `${configuredApiOrigin}/api/v1` : joinPublicApiUrl(configuredApiOrigin, '/api/v1');

  let notice: 'ok' | 'low' | 'blocked';
  const selectedBalance = creditKind === 'paid' ? balances.paid : balances.free;
  if (selectedBalance <= 0) {
    notice = 'blocked';
  } else if (costEstimate !== undefined && selectedBalance < costEstimate) {
    notice = 'low';
  } else {
    notice = 'ok';
  }

  const requestId = requestIdFromHeaders(request.headers);
  return Response.json({
    ...descriptor,
    thinkingLevel,
    api,
    provider: NEVERMIND_PROVIDER_ID,
    baseUrl,
    modelTier,
    creditKind,
    credits: { paid: balances.paid, free: balances.free, total: balances.total },
    notice,
    ...(costEstimate !== undefined ? { costEstimate } : {}),
  }, { headers: compatibilityHeaders(requestId) });
};

export const OPTIONS: APIRoute = async () => new Response(null, { status: 404 });

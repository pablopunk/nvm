import type { APIRoute } from 'astro';
import { getUserFromBearer } from '../../../lib/tokens';
import { getModelRoute, ModelNotConfiguredError, parseExtensionAiModelRole } from '../../../lib/settings';
import { ensureMonthlyFreeCredits, getBalances } from '../../../lib/users';
import { lookupModelCost, lookupModelDescriptor } from '../../../lib/pricing';
import { compatibilityHeaders, requestIdFromHeaders } from '../../../lib/compatibility';
import { selectApiForModel } from '../../../lib/upstream';
import { estimatePromptCredits } from '../../../lib/limits';
import { env } from '../../../lib/env';
import { joinPublicApiUrl, parsePublicOrigin } from '../../../../../src/shared/public-origin';

const NEVERMIND_PROVIDER_ID = 'nevermind';

export const GET: APIRoute = async ({ request }) => {
  let configuredApiOrigin: string;
  try {
    configuredApiOrigin = parsePublicOrigin(env('PUBLIC_API_ORIGIN') ?? 'https://api.nvm.fyi', 'production_api');
  } catch {
    return Response.json({ error: { type: 'configuration_error', message: 'Public API origin is unavailable.' } }, { status: 503 });
  }
  const user = await getUserFromBearer(request.headers.get('authorization'));
  if (!user) return new Response('Unauthorized', { status: 401 });
  await ensureMonthlyFreeCredits(user.id);

  const balances = await getBalances(user.id);
  const tier = balances.paid > 0 ? 'paid' : 'free';
  const url = new URL(request.url);
  const requestedModel = parseExtensionAiModelRole(request.headers.get('x-nevermind-ai-model') || url.searchParams.get('model'));
  let provider: string;
  let modelId: string;
  try {
    const route = await getModelRoute(requestedModel ?? tier);
    provider = route.provider;
    modelId = route.modelId;
  } catch (err) {
    if (err instanceof ModelNotConfiguredError) {
      return Response.json(
        { error: { type: 'model_not_configured', message: 'No active model configured.' } },
        { status: 503 },
      );
    }
    throw err;
  }

  const descriptor = await lookupModelDescriptor(provider, modelId);
  if (!descriptor) {
    return Response.json(
      { error: { type: 'model_descriptor_unavailable', message: `No descriptor for ${provider}/${modelId}` } },
      { status: 503 },
    );
  }

  const api = selectApiForModel(provider, modelId);
  const baseUrl = api === 'anthropic-messages'
    ? joinPublicApiUrl(configuredApiOrigin, '/api')
    : joinPublicApiUrl(configuredApiOrigin, '/api/v1');

  const CHARS_PER_TOKEN = 4;
  const inputTokensQ = url.searchParams.get('inputTokens');
  const charsQ = url.searchParams.get('chars');
  let costEstimate: number | undefined;
  if (inputTokensQ || charsQ) {
    const tokens = inputTokensQ
      ? Math.max(0, Number(inputTokensQ) || 0)
      : Math.ceil(Math.max(0, Number(charsQ) || 0) / CHARS_PER_TOKEN);
    if (tokens > 0) {
      const costRow = await lookupModelCost(provider, modelId);
      if (costRow) costEstimate = estimatePromptCredits(tokens, costRow);
    }
  }

  let notice: 'ok' | 'low' | 'blocked';
  if (balances.total <= 0) {
    notice = 'blocked';
  } else if (costEstimate !== undefined && balances.total < costEstimate) {
    notice = 'low';
  } else {
    notice = 'ok';
  }

  const requestId = requestIdFromHeaders(request.headers);
  return Response.json({
    ...descriptor,
    api,
    provider: NEVERMIND_PROVIDER_ID,
    baseUrl,
    credits: { paid: balances.paid, free: balances.free, total: balances.total },
    notice,
    ...(costEstimate !== undefined ? { costEstimate } : {}),
  }, { headers: compatibilityHeaders(requestId) });
};

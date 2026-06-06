import type { APIRoute } from 'astro';
import { getUserFromBearer } from '../../../lib/tokens';
import { getActiveModelId, getFreeModelId, getActiveProvider, ModelNotConfiguredError } from '../../../lib/settings';
import { ensureMonthlyFreeCredits, getBalances } from '../../../lib/users';
import { lookupModelDescriptor } from '../../../lib/pricing';
import { compatibilityHeaders, requestIdFromHeaders } from '../../../lib/compatibility';
import { selectApiForModel, type UpstreamApi } from '../../../lib/upstream';

const NEVERMIND_PROVIDER_ID = 'nevermind';

function backendBaseUrlForApi(requestUrl: URL, api: UpstreamApi): string {
  const origin = `${requestUrl.protocol}//${requestUrl.host}`;
  return api === 'anthropic-messages' ? `${origin}/api` : `${origin}/api/v1`;
}

export const GET: APIRoute = async ({ request }) => {
  const user = await getUserFromBearer(request.headers.get('authorization'));
  if (!user) return new Response('Unauthorized', { status: 401 });
  await ensureMonthlyFreeCredits(user.id);

  const balances = await getBalances(user.id);
  const usingFreeTier = balances.free > 0;
  const provider = await getActiveProvider();
  let modelId: string;
  try {
    modelId = usingFreeTier ? await getFreeModelId() : await getActiveModelId();
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
  const baseUrl = backendBaseUrlForApi(new URL(request.url), api);

  const requestId = requestIdFromHeaders(request.headers);
  return Response.json({
    ...descriptor,
    api,
    provider: NEVERMIND_PROVIDER_ID,
    baseUrl,
  }, { headers: compatibilityHeaders(requestId) });
};

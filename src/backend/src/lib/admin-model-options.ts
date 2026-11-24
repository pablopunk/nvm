import { listModelsForRouteKind, type RouteKind } from './pricing';
import { CAPABILITIES_FOR_ROUTE_KIND } from './model-capabilities/criteria';
import { activeModelCapabilityStore, passedCapabilitiesByModel } from './model-capabilities/store';
import { listEnabledProviders, modelRouteToRef } from './settings';
import { getUpstreamConfig, UpstreamConfigError } from './upstream';

export async function openRouterAccessibleModels(): Promise<Set<string> | null> {
  try {
    const { baseUrl, apiKey } = getUpstreamConfig('openrouter');
    const response = await fetch(`${baseUrl}/models`, {
      headers: { authorization: `Bearer ${apiKey}` },
      signal: AbortSignal.timeout(4000),
    });
    if (!response.ok) return null;
    const body: unknown = await response.json();
    if (!body || typeof body !== 'object' || !('data' in body) || !Array.isArray(body.data)) return null;
    const ids = body.data.map((model: unknown) =>
      model && typeof model === 'object' && 'id' in model ? model.id : null,
    );
    if (!ids.every((id: unknown) => typeof id === 'string')) return null;
    return new Set(ids as string[]);
  } catch {
    return null;
  }
}

export type CatalogCandidates = { provider: string; modelIds: string[] };

async function catalogCandidatesForProvider(provider: string, kind: RouteKind): Promise<string[]> {
  try {
    getUpstreamConfig(provider);
  } catch (error) {
    if (error instanceof UpstreamConfigError) return [];
    throw error;
  }
  const accessible = provider === 'openrouter' ? await openRouterAccessibleModels() : null;
  if (provider === 'openrouter' && !accessible) return [];
  return (await listModelsForRouteKind(provider, kind)).filter((modelId) => !accessible || accessible.has(modelId));
}

export async function listCatalogCandidates(kind: RouteKind): Promise<CatalogCandidates[]> {
  const providers = await listEnabledProviders();
  return Promise.all(
    providers.map(async ({ id }) => ({ provider: id, modelIds: await catalogCandidatesForProvider(id, kind) })),
  );
}

async function modelIdsThatPassedEveryProbe(provider: string, modelIds: string[], kind: RouteKind): Promise<string[]> {
  if (modelIds.length === 0) return [];
  const passed = passedCapabilitiesByModel(await activeModelCapabilityStore().loadProvider(provider));
  const required = CAPABILITIES_FOR_ROUTE_KIND[kind];
  return modelIds.filter((modelId) => required.every((capability) => passed.get(modelId)?.has(capability)));
}

export async function listVerifiedModelsForRouteKind(provider: string, kind: RouteKind): Promise<string[]> {
  return modelIdsThatPassedEveryProbe(provider, await listModelsForRouteKind(provider, kind), kind);
}

export async function listAdminModelRefs(kind: RouteKind): Promise<string[]> {
  const groups = await Promise.all(
    (await listCatalogCandidates(kind)).map(async ({ provider, modelIds }) =>
      (await modelIdsThatPassedEveryProbe(provider, modelIds, kind)).map((modelId) =>
        modelRouteToRef({ provider, modelId }),
      ),
    ),
  );
  return groups.flat();
}

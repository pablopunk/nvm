import { listModelsForRouteKind, type RouteKind } from './pricing';
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

export async function listAdminModelRefs(kind: RouteKind): Promise<string[]> {
  const providers = await listEnabledProviders();
  const groups = await Promise.all(
    providers.map(async ({ id }) => {
      try {
        getUpstreamConfig(id);
      } catch (error) {
        if (error instanceof UpstreamConfigError) return [];
        throw error;
      }
      const accessible = id === 'openrouter' ? await openRouterAccessibleModels() : null;
      if (id === 'openrouter' && !accessible) return [];
      return (await listModelsForRouteKind(id, kind)).filter((modelId) => !accessible || accessible.has(modelId)).map((modelId) =>
        modelRouteToRef({ provider: id, modelId }),
      );
    }),
  );
  return groups.flat();
}

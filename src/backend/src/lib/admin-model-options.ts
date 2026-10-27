import { listModelsForRouteKind, type RouteKind } from './pricing';
import { listEnabledProviders, modelRouteToRef } from './settings';
import { getUpstreamConfig, UpstreamConfigError } from './upstream';

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
      return (await listModelsForRouteKind(id, kind)).map((modelId) =>
        modelRouteToRef({ provider: id, modelId }),
      );
    }),
  );
  return groups.flat();
}

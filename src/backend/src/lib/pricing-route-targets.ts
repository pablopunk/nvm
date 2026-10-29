import { pricingContractForProvider } from './provider-pricing-contract';
import {
  getModelProviderRoutes,
  getModelRoute,
  ModelNotConfiguredError,
  type ModelRouteConfig,
  type TieredModelRouteSlot,
} from './settings';
import { liveTieredRouteSlots } from './upstream-route-verdicts';

/**
 * The OpenRouter models the live tiered routes may call: each primary and its
 * failover links, as the request path would reach them. Audio and decision routes
 * price through their own catalogs and are not listed here.
 */
export async function openRouterRouteModelIds(): Promise<string[]> {
  const modelIds = new Set<string>();
  for (const slot of liveTieredRouteSlots()) {
    const primary = await liveRouteOrNull(slot);
    if (!primary) continue;
    const links = await getModelProviderRoutes(slot, primary.modelId);
    for (const route of [{ providerId: primary.provider, modelId: primary.modelId }, ...links])
      if (isOpenRouterPriced(route.providerId)) modelIds.add(route.modelId);
  }
  return [...modelIds].sort();
}

async function liveRouteOrNull(
  slot: TieredModelRouteSlot,
): Promise<ModelRouteConfig | null> {
  try {
    return await getModelRoute(slot);
  } catch (error) {
    if (error instanceof ModelNotConfiguredError) return null;
    throw error;
  }
}

function isOpenRouterPriced(provider: string): boolean {
  return (
    pricingContractForProvider(provider).priceDiscovery === 'openrouter_endpoints'
  );
}

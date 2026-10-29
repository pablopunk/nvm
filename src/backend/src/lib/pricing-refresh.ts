import { refreshDecisionCatalog } from './decisions/decision-models';
import { log } from './log';
import { refreshModelsDevCatalog } from './pricing-catalog';
import { refreshOpenRouterEndpointPrice } from './pricing-openrouter';
import { pricingContractForProvider } from './provider-pricing-contract';
import { refreshTranscriptionCatalog } from './transcription-models';

type CatalogOutcome = 'refreshed' | 'failed';

export type PricingRefreshReport = {
  catalog: CatalogOutcome;
  openRouter: { refreshed: string[]; failed: string[] };
  /** The OpenRouter audio and decision catalogs, each one durable snapshot. */
  openRouterCatalogs: { transcription: CatalogOutcome; decision: CatalogOutcome };
};

/** Whether every price and catalog the request path reads was validated by this refresh. */
export function pricingRefreshSucceeded(report: PricingRefreshReport): boolean {
  return (
    report.catalog === 'refreshed' &&
    report.openRouter.failed.length === 0 &&
    report.openRouterCatalogs.transcription === 'refreshed' &&
    report.openRouterCatalogs.decision === 'refreshed'
  );
}

function outcomeOf(refresh: () => Promise<void>): Promise<CatalogOutcome> {
  return refresh().then(
    () => 'refreshed' as const,
    () => 'failed' as const,
  );
}

/**
 * Refreshes every validated price and catalog the request path reads: the models.dev sections,
 * the endpoint listing of each live OpenRouter route model, and the OpenRouter audio and
 * decision catalogs. Each OpenRouter model is isolated, so one failing endpoint never blocks
 * the others.
 */
export async function refreshPricingSnapshots(
  openRouterModelIds: readonly string[],
): Promise<PricingRefreshReport> {
  const modelIds = [...new Set(openRouterModelIds)];
  // Independent fetches run concurrently, so the whole refresh takes one timeout, not several.
  const [catalog, outcomes, transcription, decision] = await Promise.all([
    refreshModelsDevCatalog().then(
      () => 'refreshed' as const,
      (error: unknown) => {
        log.warn('pricing_catalog_refresh_failed', { error: String(error) });
        return 'failed' as const;
      },
    ),
    Promise.all(modelIds.map(refreshEndpointOutcome)),
    outcomeOf(refreshTranscriptionCatalog),
    outcomeOf(refreshDecisionCatalog),
  ]);
  return {
    catalog,
    openRouter: {
      refreshed: outcomes.filter((o) => o.ok).map((o) => o.modelId),
      failed: outcomes.filter((o) => !o.ok).map((o) => o.modelId),
    },
    openRouterCatalogs: { transcription, decision },
  };
}

async function refreshEndpointOutcome(
  modelId: string,
): Promise<{ modelId: string; ok: boolean }> {
  try {
    await refreshOpenRouterEndpointPrice(modelId);
    return { modelId, ok: true };
  } catch (error) {
    log.warn('pricing_endpoint_refresh_failed', {
      model: modelId,
      error: String(error),
    });
    return { modelId, ok: false };
  }
}

/**
 * Admin path only: validates the price of one route model before the route is saved.
 * It fetches at most the one catalog or endpoint that route reads, and never throws,
 * because the caller refuses the route when the price is still unavailable.
 */
export async function refreshRoutePricing(
  provider: string,
  modelId: string,
): Promise<boolean> {
  const contract = pricingContractForProvider(provider);
  try {
    if (contract.priceDiscovery === 'openrouter_endpoints')
      await refreshOpenRouterEndpointPrice(modelId);
    else if (contract.priceDiscovery === 'models_dev')
      await refreshModelsDevCatalog();
    return true;
  } catch (error) {
    log.warn('pricing_route_refresh_failed', {
      provider,
      model: modelId,
      error: String(error),
    });
    return false;
  }
}

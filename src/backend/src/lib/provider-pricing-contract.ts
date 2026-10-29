/**
 * Declares, per upstream provider, where prices come from, where capabilities
 * come from, and whether the upstream response's reported cost may settle a
 * request. Proxy, settlement, and decision billing all consult this one contract.
 */
export type PriceDiscovery = 'models_dev' | 'openrouter_endpoints' | 'none';
export type CapabilitySource = 'models_dev' | 'none';
/**
 * `usage_cost_usd`: the response's `usage.cost` is the upstream's own charge and
 * may settle a request in place of the catalog estimate. Its units are assumed to
 * be USD, as the existing settlement path has always assumed; live verification
 * of that unit is still outstanding.
 */
export type ReportedCostSemantics = 'usage_cost_usd' | 'none';

export type ProviderPricingContract = {
  priceDiscovery: PriceDiscovery;
  capabilitySource: CapabilitySource;
  reportedCost: ReportedCostSemantics;
  /** Key of the provider's section in the models.dev catalog. */
  catalogKey: string;
};

const PROVIDER_PRICING_CONTRACTS: Record<string, ProviderPricingContract> = {
  opencode_zen: {
    priceDiscovery: 'models_dev',
    capabilitySource: 'models_dev',
    reportedCost: 'none',
    catalogKey: 'opencode',
  },
  opencode: {
    priceDiscovery: 'models_dev',
    capabilitySource: 'models_dev',
    reportedCost: 'none',
    catalogKey: 'opencode',
  },
  openrouter: {
    // Routing may pick any live endpoint, so admission prices come from the
    // per-endpoint OpenRouter pricing, never the single model-level price.
    priceDiscovery: 'openrouter_endpoints',
    capabilitySource: 'models_dev',
    reportedCost: 'usage_cost_usd',
    catalogKey: 'openrouter',
  },
  anthropic: {
    priceDiscovery: 'models_dev',
    capabilitySource: 'models_dev',
    reportedCost: 'none',
    catalogKey: 'anthropic',
  },
  google: {
    priceDiscovery: 'models_dev',
    capabilitySource: 'models_dev',
    reportedCost: 'none',
    catalogKey: 'google',
  },
  openai: {
    priceDiscovery: 'models_dev',
    capabilitySource: 'models_dev',
    reportedCost: 'none',
    catalogKey: 'openai',
  },
};

const UNDECLARED_PROVIDER_CONTRACT: ProviderPricingContract = {
  priceDiscovery: 'none',
  capabilitySource: 'none',
  reportedCost: 'none',
  catalogKey: '',
};

/** Undeclared providers fail closed: no catalog section is read for them. */
export function pricingContractForProvider(
  provider: string,
): ProviderPricingContract {
  return PROVIDER_PRICING_CONTRACTS[provider] ?? UNDECLARED_PROVIDER_CONTRACT;
}

/** Every models.dev section any declared provider reads, each listed once. */
export function declaredCatalogKeys(): string[] {
  return [
    ...new Set(
      Object.values(PROVIDER_PRICING_CONTRACTS).map(
        (contract) => contract.catalogKey,
      ),
    ),
  ];
}

/**
 * The only path by which an upstream-reported cost may settle a request. A
 * provider whose contract does not declare reported cost semantics is always
 * settled from the catalog estimate, whatever the response carries.
 */
export function trustedReportedCostUsd(
  provider: string,
  reportedCostUsd: number | null | undefined,
): number | undefined {
  if (pricingContractForProvider(provider).reportedCost !== 'usage_cost_usd')
    return undefined;
  return reportedCostUsd != null &&
    Number.isFinite(reportedCostUsd) &&
    reportedCostUsd >= 0
    ? reportedCostUsd
    : undefined;
}

export type AccountFundingChoice = {
  tier: 'free' | 'pro';
  funded: boolean;
  freeFallback: boolean;
};

/** Decide route display from entitlement and the same combined funding pool used at admission. */
export function chooseFundedAccountRoute(input: {
  tier: 'free' | 'pro';
  freeCredits: number;
  paidCredits: number;
  routeCredits: number | undefined;
  freeRouteCredits?: number;
  freeFallbackAllowed?: boolean;
}): AccountFundingChoice {
  const healthyBalances = input.freeCredits >= 0 && input.paidCredits >= 0;
  const available = input.freeCredits + input.paidCredits;
  const funded =
    healthyBalances &&
    input.routeCredits !== undefined &&
    available >= input.routeCredits;
  if (funded || input.tier !== 'pro' || !input.freeFallbackAllowed) {
    return { tier: input.tier, funded, freeFallback: false };
  }

  const freeFallback =
    healthyBalances &&
    input.freeRouteCredits !== undefined &&
    available >= input.freeRouteCredits;
  return {
    tier: freeFallback ? 'free' : 'pro',
    funded: freeFallback,
    freeFallback,
  };
}

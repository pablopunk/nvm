function isPositiveSafeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0;
}

export const DEFAULT_NEVERMIND_MAX_INPUT_TOKENS = 100_000;

export function effectiveNevermindContextWindow(
  providerContextWindow: number,
  maxInputTokens?: number,
) {
  const safeProviderContextWindow = isPositiveSafeInteger(providerContextWindow)
    ? providerContextWindow
    : DEFAULT_NEVERMIND_MAX_INPUT_TOKENS;
  const safeMaxInputTokens = isPositiveSafeInteger(maxInputTokens)
    ? maxInputTokens
    : DEFAULT_NEVERMIND_MAX_INPUT_TOKENS;
  return Math.min(safeProviderContextWindow, safeMaxInputTokens);
}

import { computeUsdCost, usdToCredits, type ModelCost } from './cost';

export const MAX_INPUT_TOKENS = Number(process.env.MAX_INPUT_TOKENS ?? 100_000);

const CHARS_PER_TOKEN = 4;

function collectStrings(node: unknown, out: string[]): void {
  if (typeof node === 'string') {
    out.push(node);
    return;
  }
  if (Array.isArray(node)) {
    for (const v of node) collectStrings(v, out);
    return;
  }
  if (node && typeof node === 'object') {
    for (const v of Object.values(node)) collectStrings(v, out);
  }
}

export function estimateInputTokensFromBody(bodyText: string): number {
  if (!bodyText) return 0;
  let parsed: unknown;
  try { parsed = JSON.parse(bodyText); } catch { return Math.ceil(bodyText.length / CHARS_PER_TOKEN); }
  const buckets: string[] = [];
  collectStrings(parsed, buckets);
  const chars = buckets.reduce((acc, s) => acc + s.length, 0);
  return Math.ceil(chars / CHARS_PER_TOKEN);
}

export function estimatePromptCredits(inputTokens: number, cost: ModelCost): number {
  return usdToCredits(computeUsdCost(cost, inputTokens, 0));
}

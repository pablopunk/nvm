/**
 * Shared fuzzy-match scoring core.
 *
 * Both arguments must already be lowercased and trimmed by the caller.
 * `query` must be non‑empty; wrappers guard that contract.
 */
export function scoreNormalizedNonEmpty(text: string, query: string): number {
  if (text === query) return 100;
  if (text.startsWith(query)) return 80;
  if (text.includes(query)) return 50;
  let pos = 0;
  for (const ch of query) {
    pos = text.indexOf(ch, pos);
    if (pos === -1) return 0;
    pos += 1;
  }
  return 20;
}

/**
 * Fuzzy-match scoring with character-set fallback for root search.
 *
 * After sequential matching fails, falls back to character-count matching:
 * every query character must appear somewhere in the text (any order),
 * with multiplicity respected. This catches typos like "gmial" → "Gmail"
 * where characters are in the wrong order.
 *
 * Both arguments must already be lowercased and trimmed by the caller.
 * `query` must be non‑empty; wrappers guard that contract.
 *
 * Score bands:
 *   100 — exact match
 *    80 — starts with
 *    50 — contains (substring)
 *    20 — sequential character match (original fuzzy)
 *    10 — character-set match (all query chars exist, any order)
 *     0 — no match
 */
export function scoreFuzzy(text: string, query: string): number {
  const sequential = scoreNormalizedNonEmpty(text, query);
  if (sequential > 0) return sequential;

  // Character-count fallback: ensure text has at least as many of each
  // query character as the query itself, regardless of order.
  const textCounts = new Map<string, number>();
  for (const ch of text) {
    textCounts.set(ch, (textCounts.get(ch) || 0) + 1);
  }
  for (const ch of query) {
    const remaining = textCounts.get(ch) || 0;
    if (remaining <= 0) return 0;
    textCounts.set(ch, remaining - 1);
  }
  return 10;
}

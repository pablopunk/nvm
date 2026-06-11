/**
 * Shared fuzzy-match scoring core.
 *
 * Both arguments must already be lowercased and trimmed by the caller.
 * `query` must be non‑empty; wrappers guard that contract.
 */
export function scoreNormalizedNonEmpty(text: string, query: string): number {
  if (text === query) return 100
  if (text.startsWith(query)) return 80
  if (text.includes(query)) return 50
  let pos = 0
  for (const ch of query) {
    pos = text.indexOf(ch, pos)
    if (pos === -1) return 0
    pos += 1
  }
  return 20
}

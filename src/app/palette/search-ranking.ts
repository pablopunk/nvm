// biome-ignore-all lint/style/noMagicNumbers: score bands are part of the public ranking contract.
/**
 * Shared fuzzy-match scoring core.
 *
 * Both arguments must already be lowercased and trimmed by the caller.
 * `query` must be non‑empty; wrappers guard that contract.
 */
const WORD_CHARACTER = /[\p{L}\p{N}]/u;

function words(value: string) {
  return value.split(/[^\p{L}\p{N}]+/u).filter(Boolean);
}

function queryWordPrefixScore(text: string, query: string) {
  const queryWords = words(query);
  if (queryWords.length < 2) return 0;
  const textWords = words(text);
  const orderedMatchIndexes: number[] = [];
  let nextTextWordIndex = 0;
  for (const queryWord of queryWords) {
    const relativeIndex = textWords
      .slice(nextTextWordIndex)
      .findIndex((textWord) => textWord.startsWith(queryWord));
    if (relativeIndex < 0) break;
    nextTextWordIndex += relativeIndex;
    orderedMatchIndexes.push(nextTextWordIndex);
    nextTextWordIndex += 1;
  }
  if (orderedMatchIndexes.length === queryWords.length)
    return orderedMatchIndexes[0] === 0 ? 95 : 70;

  const availableTextWords = [...textWords];
  for (const queryWord of queryWords) {
    const matchIndex = availableTextWords.findIndex((textWord) =>
      textWord.startsWith(queryWord),
    );
    if (matchIndex < 0) return 0;
    availableTextWords.splice(matchIndex, 1);
  }
  return 60;
}

export function scoreNormalizedNonEmpty(text: string, query: string): number {
  if (text === query) {
    return 100;
  }
  // Check this before startsWith so a leading complete word gets the same
  // stronger word-match score as a word later in the title.
  const isWordBoundary = (value: string | undefined) =>
    value === undefined || !WORD_CHARACTER.test(value);
  let phraseScore = 0;
  let start = text.indexOf(query);
  while (start >= 0) {
    if (
      isWordBoundary(text[start - 1]) &&
      isWordBoundary(text[start + query.length])
    ) {
      phraseScore = 90;
      break;
    }
    start = text.indexOf(query, start + 1);
  }
  phraseScore = Math.max(
    phraseScore,
    text.startsWith(query) ? 80 : 0,
    text.includes(query) ? 50 : 0,
    queryWordPrefixScore(text, query),
  );
  if (phraseScore > 0) return phraseScore;
  let pos = 0;
  for (const ch of query) {
    pos = text.indexOf(ch, pos);
    if (pos === -1) {
      return 0;
    }
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
 *   100 - exact match
 *    95 - ordered word-prefix match starting at the first title word
 *    90 - whole-word phrase match
 *    80 - starts with
 *    70 - ordered word-prefix match
 *    60 - unordered word-prefix match
 *    50 - contains (substring)
 *    20 - sequential character match (original fuzzy)
 *    10 - character-set match (all query chars exist, any order)
 *     0 - no match
 */
export function scoreFuzzy(text: string, query: string): number {
  const sequential = scoreNormalizedNonEmpty(text, query);
  if (sequential > 0) {
    return sequential;
  }

  // Character-count fallback: ensure text has at least as many of each
  // query character as the query itself, regardless of order.
  const textCounts = new Map<string, number>();
  for (const ch of text) {
    textCounts.set(ch, (textCounts.get(ch) || 0) + 1);
  }
  for (const ch of query) {
    const remaining = textCounts.get(ch) || 0;
    if (remaining <= 0) {
      return 0;
    }
    textCounts.set(ch, remaining - 1);
  }
  return 10;
}

import {
  CHARACTER_RECORDS,
  type CharacterCatalogRecord,
} from '../electron/app/emoji-symbol-catalog';
import type {
  ExtensionAi,
  ExtensionDecisionAnswer,
  ExtensionDecisionQuestion,
} from '../resources/nevermind-extension-api';

/**
 * Meaning-based second batch for the character picker. The decision API answers at
 * most 16 questions per call, so the 3,800-character catalog is narrowed in steps:
 *
 * 1. Category predicates choose which parts of the catalog could match.
 * 2. Choice questions (31 candidates plus "none of these" each) shortlist candidates.
 * 3. One predicate per shortlisted candidate gives the 0..1 score ranked and cut at
 *    `MATCH_THRESHOLD`.
 */

/** A shortlisted candidate must be at least this likely to be a match to be shown. */
export const MATCH_THRESHOLD = 0.5;
const CATEGORY_THRESHOLD = 0.3;
const MIN_SHORTLIST_PROBABILITY = 0.02;
const MAX_QUESTIONS = 16;
const CHOICES_PER_QUESTION = 31;
const NONE_OF_THESE = 'none of these';
const RECALL_POOL_LIMIT = MAX_QUESTIONS * CHOICES_PER_QUESTION;
const MIN_QUERY_LENGTH = 3;
const MAX_QUERY_LENGTH = 80;
const DECISION_TIMEOUT_MS = 8000;
const CACHE_LIMIT = 32;
// Flags are matched by country name, and Technical/Math are matched by code point or name.
const LEXICAL_ONLY_GROUPS = new Set(['Flags', 'Technical', 'Math']);

type DecisionAi = Pick<ExtensionAi, 'decide'>;

const RECORDS_BY_GROUP = new Map<string, CharacterCatalogRecord[]>();
for (const record of CHARACTER_RECORDS) {
  if (LEXICAL_ONLY_GROUPS.has(record.group)) continue;
  RECORDS_BY_GROUP.set(record.group, [
    ...(RECORDS_BY_GROUP.get(record.group) ?? []),
    record,
  ]);
}
const SEMANTIC_GROUPS = [...RECORDS_BY_GROUP.keys()].slice(0, MAX_QUESTIONS);

const matchCache = new Map<string, CharacterCatalogRecord[]>();

function optionLabel(record: CharacterCatalogRecord) {
  return `${record.glyph} ${record.name}`;
}

function searchInput(query: string) {
  return `Search text: ${JSON.stringify(query)}`;
}

function decide(
  ai: DecisionAi,
  query: string,
  questions: ExtensionDecisionQuestion[],
) {
  return ai.decide({
    input: searchInput(query),
    questions,
    signal: AbortSignal.timeout(DECISION_TIMEOUT_MS),
  });
}

function predicateProbability(answer: ExtensionDecisionAnswer | undefined) {
  return answer?.type === 'predicate' ? answer.probability : 0;
}

async function likelyGroups(ai: DecisionAi, query: string, category: string) {
  if (category !== 'all')
    return SEMANTIC_GROUPS.includes(category) ? [category] : [];
  const result = await decide(
    ai,
    query,
    SEMANTIC_GROUPS.map((group, index) => ({
      id: `group_${index}`,
      type: 'predicate' as const,
      question: `Would an emoji or symbol from the "${group}" category be a good result for this search text?`,
    })),
  );
  return SEMANTIC_GROUPS.map((group, index) => ({
    group,
    probability: predicateProbability(result.answers[`group_${index}`]),
  }))
    .filter((entry) => entry.probability >= CATEGORY_THRESHOLD)
    .sort((a, b) => b.probability - a.probability)
    .map((entry) => entry.group);
}

function recallPool(groups: string[]) {
  const seen = new Set<string>();
  return groups
    .flatMap((group) => RECORDS_BY_GROUP.get(group) ?? [])
    .filter((record) => {
      const label = optionLabel(record);
      if (seen.has(label)) return false;
      seen.add(label);
      return true;
    })
    .slice(0, RECALL_POOL_LIMIT);
}

function chunks<T>(items: T[], size: number) {
  const result: T[][] = [];
  for (let start = 0; start < items.length; start += size)
    result.push(items.slice(start, start + size));
  return result;
}

function choiceCandidates(
  answer: ExtensionDecisionAnswer | undefined,
): Array<[label: string, probability: number]> {
  if (answer?.type !== 'choice') return [];
  if (answer.probabilities) return Object.entries(answer.probabilities);
  return [[answer.value, answer.confidence ?? MATCH_THRESHOLD]];
}

async function shortlist(
  ai: DecisionAi,
  query: string,
  pool: CharacterCatalogRecord[],
) {
  const poolChunks = chunks(pool, CHOICES_PER_QUESTION);
  const result = await decide(
    ai,
    query,
    poolChunks.map((chunk, index) => ({
      id: `recall_${index}`,
      type: 'choice' as const,
      question: 'Which option is the best match for the search text?',
      options: [...chunk.map(optionLabel), NONE_OF_THESE],
    })),
  );
  const recordsByLabel = new Map(
    pool.map((record) => [optionLabel(record), record]),
  );
  return poolChunks
    .flatMap((_chunk, index) =>
      choiceCandidates(result.answers[`recall_${index}`]),
    )
    .filter(
      ([label, probability]) =>
        label !== NONE_OF_THESE && probability >= MIN_SHORTLIST_PROBABILITY,
    )
    .sort((a, b) => b[1] - a[1])
    .slice(0, MAX_QUESTIONS)
    .flatMap(([label]) => recordsByLabel.get(label) ?? []);
}

async function verifiedMatches(
  ai: DecisionAi,
  query: string,
  candidates: CharacterCatalogRecord[],
) {
  if (candidates.length === 0) return [];
  const result = await decide(
    ai,
    query,
    candidates.map((record, index) => ({
      id: `match_${index}`,
      type: 'predicate' as const,
      question: `Is ${optionLabel(record)} a good match for the search text?`,
    })),
  );
  return candidates
    .map((record, index) => ({
      record,
      probability: predicateProbability(result.answers[`match_${index}`]),
    }))
    .filter((entry) => entry.probability >= MATCH_THRESHOLD)
    .sort((a, b) => b.probability - a.probability)
    .map((entry) => entry.record);
}

/** Characters the decision model ranks as matches for the text, best first. */
export async function semanticCharacterMatches(
  ai: DecisionAi,
  category: string,
  searchText: string,
) {
  const query = searchText.trim().toLowerCase().slice(0, MAX_QUERY_LENGTH);
  if (query.length < MIN_QUERY_LENGTH) return [];
  const cacheKey = `${category}\u0000${query}`;
  const cached = matchCache.get(cacheKey);
  if (cached) return cached;
  const pool = recallPool(await likelyGroups(ai, query, category));
  const matches =
    pool.length === 0
      ? []
      : await verifiedMatches(ai, query, await shortlist(ai, query, pool));
  matchCache.set(cacheKey, matches);
  if (matchCache.size > CACHE_LIMIT)
    matchCache.delete(matchCache.keys().next().value as string);
  return matches;
}

import { scoreNormalizedNonEmpty } from '../../../../app/palette/search-ranking';
import type { PaletteItem } from './palette-types';

export const MAX_RESULTS = 12;
const ALIAS_WEIGHT = 0.9;
const KEYWORD_WEIGHT = 0.7;
const SUBTITLE_WEIGHT = 0.5;

function weightedFields(item: PaletteItem): Array<[string, number]> {
  return [
    [item.title, 1],
    ...item.aliases.map((alias): [string, number] => [alias, ALIAS_WEIGHT]),
    ...(item.keywords ?? []).map((word): [string, number] => [
      word,
      KEYWORD_WEIGHT,
    ]),
    [item.subtitle, SUBTITLE_WEIGHT],
  ];
}

function scoreItem(item: PaletteItem, query: string) {
  return Math.max(
    ...weightedFields(item).map(([text, weight]) =>
      text ? scoreNormalizedNonEmpty(text.toLowerCase(), query) * weight : 0,
    ),
  );
}

/**
 * Ranks items with the desktop app's own fuzzy scorer, so the landing palette
 * orders results the way the real one does. An empty query keeps list order.
 */
export function searchPalette(items: PaletteItem[], query: string) {
  const needle = query.trim().toLowerCase();
  if (!needle) return items.slice(0, MAX_RESULTS);
  return items
    .map((item, order) => ({ item, order, score: scoreItem(item, needle) }))
    .filter((match) => match.score > 0)
    .sort((a, b) => b.score - a.score || a.order - b.order)
    .slice(0, MAX_RESULTS)
    .map((match) => match.item);
}

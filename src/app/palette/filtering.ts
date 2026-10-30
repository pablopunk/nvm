import { actionsFromPanel, type CommandItem, type CommandView } from './model';
import { scoreNormalizedNonEmpty } from './search-ranking';

export function scoreText(value: string | undefined, filter: string): number {
  if (!filter) return 1;
  const text = (value || '').toLowerCase();
  return scoreNormalizedNonEmpty(text, filter);
}

export function valuesMatch(
  filterValue: string,
  ...values: Array<string | undefined>
) {
  const filter = filterValue.trim().toLowerCase();
  if (!filter) return true;
  return Math.max(...values.map((value) => scoreText(value, filter))) > 0;
}

export function valuesMatchMinScore(
  minScore: number,
  filterValue: string,
  ...values: Array<string | undefined>
) {
  const filter = filterValue.trim().toLowerCase();
  if (!filter) return true;
  return (
    Math.max(...values.map((value) => scoreText(value, filter))) >= minScore
  );
}

export function allViewItems(view: CommandView) {
  return view.sections?.flatMap((section) => section.items) || view.items || [];
}

function commandItemSearchValues(item: CommandItem) {
  return [
    item.title,
    item.subtitle,
    item.text,
    item.shortcut,
    ...(item.keywords || []),
    ...actionsFromPanel(item.actionPanel, item.actions || []).map(
      (action) => action.title,
    ),
  ];
}

const EXACT_TITLE_MATCH_RANK = 0;
const EXACT_KEYWORD_MATCH_RANK = 1;
const PARTIAL_MATCH_RANK = 2;

function normalizedSearchValue(value: string | undefined) {
  return (value || '').trim().toLowerCase();
}

function exactMatchRank(item: CommandItem, filter: string) {
  if (normalizedSearchValue(item.title) === filter)
    return EXACT_TITLE_MATCH_RANK;
  if (
    (item.keywords || []).some(
      (value) => normalizedSearchValue(value) === filter,
    )
  )
    return EXACT_KEYWORD_MATCH_RANK;
  return PARTIAL_MATCH_RANK;
}

function exactMatchesFirst(items: CommandItem[], filterValue: string) {
  const filter = normalizedSearchValue(filterValue);
  if (!filter) return items;
  return items
    .map((item, index) => ({ item, index, rank: exactMatchRank(item, filter) }))
    .sort((left, right) => left.rank - right.rank || left.index - right.index)
    .map(({ item }) => item);
}

export function filterCommandItems(
  items: CommandItem[] = [],
  filter: string,
  options?: { minScore?: number },
) {
  const minScore = options?.minScore ?? 1;
  const matches = items.filter((item) =>
    valuesMatchMinScore(minScore, filter, ...commandItemSearchValues(item)),
  );
  return exactMatchesFirst(matches, filter);
}

export function filterCommandSections(
  view: CommandView,
  filter: string,
  options?: { minScore?: number },
) {
  if (!view.sections?.length) return;
  return view.sections
    .map((section) => ({
      ...section,
      items: filterCommandItems(section.items, filter, options),
    }))
    .filter((section) => section.items.length > 0);
}

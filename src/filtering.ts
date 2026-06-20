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

export function filterCommandItems(
  items: CommandItem[] = [],
  filter: string,
  options?: { minScore?: number },
) {
  const minScore = options?.minScore ?? 1;
  return items.filter((item) =>
    valuesMatchMinScore(
      minScore,
      filter,
      item.title,
      item.subtitle,
      item.text,
      item.shortcut,
      ...(item.keywords || []),
      ...actionsFromPanel(item.actionPanel, item.actions || []).map(
        (action) => action.title,
      ),
    ),
  );
}

export function filterCommandSections(
  view: CommandView,
  filter: string,
  options?: { minScore?: number },
) {
  if (!view.sections?.length) return undefined;
  return view.sections
    .map((section) => ({
      ...section,
      items: filterCommandItems(section.items, filter, options),
    }))
    .filter((section) => section.items.length > 0);
}

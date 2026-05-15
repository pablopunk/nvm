import { actionsFromPanel, type CommandItem, type CommandView } from './model'

export function scoreText(value: string | undefined, filter: string) {
  const text = value?.toLowerCase() || ''
  if (!filter) return 1
  if (text === filter) return 100
  if (text.startsWith(filter)) return 80
  if (text.includes(filter)) return 50
  let position = 0
  for (const character of filter) {
    position = text.indexOf(character, position)
    if (position === -1) return 0
    position += 1
  }
  return 20
}

export function valuesMatch(filterValue: string, ...values: Array<string | undefined>) {
  const filter = filterValue.trim().toLowerCase()
  if (!filter) return true
  return Math.max(...values.map((value) => scoreText(value, filter))) > 0
}

export function allViewItems(view: CommandView) {
  return view.sections?.flatMap((section) => section.items) || view.items || []
}

export function filterCommandItems(items: CommandItem[] = [], filter: string) {
  return items.filter((item) => valuesMatch(
    filter,
    item.title,
    item.subtitle,
    item.text,
    ...(item.keywords || []),
    ...actionsFromPanel(item.actionPanel, item.actions || []).map((action) => action.title),
  ))
}

export function filterCommandSections(view: CommandView, filter: string) {
  if (!view.sections?.length) return undefined
  return view.sections.map((section) => ({ ...section, items: filterCommandItems(section.items, filter) })).filter((section) => section.items.length > 0)
}

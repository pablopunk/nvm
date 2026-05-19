import type { ReactNode } from 'react'
import { Search, Sparkles } from 'lucide-react'
import { CommandRow, EmptyState, EMPTY_ROOT_TITLE, EMPTY_ROOT_SUBTITLE } from './ui'
import { actionsFromPanel, type CommandAction, type CommandItem } from './model'

export type RootCommandListProps = {
  items: CommandItem[]
  iconForItem: (item: CommandItem) => ReactNode
  onSelect: (item: CommandItem) => void
  emptyTitle?: string
  emptySubtitle?: string
  extraForItem?: (item: CommandItem) => string[]
}

function isGlobalShortcut(action?: CommandAction) {
  return action?.shortcutScope === 'global' || action?.type === 'nativeAction'
}

export function RootCommandList({ items, iconForItem, onSelect, emptyTitle = EMPTY_ROOT_TITLE, emptySubtitle = EMPTY_ROOT_SUBTITLE, extraForItem }: RootCommandListProps) {
  if (items.length === 0) return <EmptyState icon={<Search size={24} />} title={emptyTitle} subtitle={emptySubtitle} />
  return <>{items.map((item) => {
    const primaryAction = item.primaryAction || actionsFromPanel(item.actionPanel, item.actions || [])[0]
    return <CommandRow
      key={item.id}
      value={item.id}
      icon={iconForItem(item) || <Sparkles size={18} />}
      title={item.title}
      subtitle={item.subtitle || item.text}
      accessories={item.accessories}
      shortcut={primaryAction?.shortcut}
      appearance={item.appearance}
      extras={extraForItem?.(item)}
      selectedOnlyShortcut={Boolean(primaryAction?.shortcut && !isGlobalShortcut(primaryAction))}
      onSelect={() => onSelect(item)}
    />
  })}</>
}

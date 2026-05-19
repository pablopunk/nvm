import * as LucideIcons from 'lucide-react'
import type { ComponentType } from 'react'
import type { CommandAction, CommandItem } from './model'

type LucideComponent = ComponentType<{ size?: number; className?: string }>

const curatedIconAliases = {
  app: 'AppWindow',
  bolt: 'Zap',
  calculator: 'Calculator',
  clipboard: 'Clipboard',
  folder: 'Folder',
  globe: 'Globe',
  grid: 'Grid2X2',
  keyboard: 'Keyboard',
  lock: 'Lock',
  moon: 'Moon',
  power: 'Power',
  restart: 'RotateCcw',
  search: 'Search',
  settings: 'Settings',
  sparkles: 'Sparkles',
  tag: 'Tag',
  trash: 'Trash2',
} as const

export const iconFor = Object.fromEntries(
  Object.entries(curatedIconAliases).map(([name, lucideName]) => [name, LucideIcons[lucideName] as LucideComponent]),
) as Record<keyof typeof curatedIconAliases, LucideComponent>

export type CommandIconName = string

function pascalCaseIconName(name: string) {
  return name
    .replace(/Icon$/, '')
    .split(/[^a-zA-Z0-9]+/)
    .filter(Boolean)
    .map((part) => `${part.slice(0, 1).toUpperCase()}${part.slice(1)}`)
    .join('')
}

function lucideIcon(name: unknown, fallback: keyof typeof curatedIconAliases = 'sparkles') {
  const requested = String(name || '').trim()
  const alias = curatedIconAliases[requested as keyof typeof curatedIconAliases]
  const candidates = [alias, requested, pascalCaseIconName(requested), `${pascalCaseIconName(requested)}Icon`].filter(Boolean)
  for (const candidate of candidates) {
    const Icon = (LucideIcons as Record<string, unknown>)[candidate]
    if (typeof Icon === 'object' || typeof Icon === 'function') return Icon as LucideComponent
  }
  return iconFor[fallback]
}

export function iconForAction(action: CommandAction) {
  if (action.type === 'copyText' || action.type === 'copyImage' || action.type === 'pasteText') return <LucideIcons.Clipboard size={18} />
  if (action.type === 'trash') return <LucideIcons.Trash2 size={18} />
  if (action.type === 'revealPath' || action.type === 'openPath' || action.type === 'quickLook' || action.type === 'openWith') return <LucideIcons.Folder size={18} />
  if (action.type === 'nativeAction') return <LucideIcons.Sparkles size={18} />
  return <LucideIcons.Globe size={18} />
}

export function iconForItem(item: CommandItem, fallback: CommandIconName = 'sparkles') {
  const Icon = lucideIcon(item.icon, fallback as keyof typeof curatedIconAliases)
  return item.image ? <span className="thumbnailIcon"><img src={item.image} alt="" /></span> : <Icon size={18} />
}

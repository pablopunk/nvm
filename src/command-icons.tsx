import { AppWindow, Calculator, Clipboard, Folder, Globe, Grid2X2, Keyboard, Lock, Moon, RotateCcw, Search, Settings, Sparkles, Tag, Trash2, Zap, Power } from 'lucide-react'
import type { CommandAction, CommandItem } from './model'

export const iconFor = {
  globe: Globe,
  search: Search,
  app: AppWindow,
  clipboard: Clipboard,
  sparkles: Sparkles,
  lock: Lock,
  moon: Moon,
  restart: RotateCcw,
  settings: Settings,
  folder: Folder,
  power: Power,
  calculator: Calculator,
  bolt: Zap,
  grid: Grid2X2,
  keyboard: Keyboard,
  tag: Tag,
  trash: Trash2,
}

export type CommandIconName = keyof typeof iconFor

export function iconForAction(action: CommandAction) {
  if (action.type === 'copyText' || action.type === 'copyImage' || action.type === 'pasteText') return <Clipboard size={18} />
  if (action.type === 'trash') return <Trash2 size={18} />
  if (action.type === 'revealPath' || action.type === 'openPath' || action.type === 'quickLook' || action.type === 'openWith') return <Folder size={18} />
  if (action.type === 'nativeAction') return <Sparkles size={18} />
  return <Globe size={18} />
}

export function iconForItem(item: CommandItem, fallback: CommandIconName = 'sparkles') {
  const Icon = iconFor[(item.icon as CommandIconName) || fallback] ?? Sparkles
  return item.image ? <span className="thumbnailIcon"><img src={item.image} alt="" /></span> : <Icon size={18} />
}

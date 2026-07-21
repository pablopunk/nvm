import * as LucideIcons from 'lucide-react';
import React, { type ComponentType } from 'react';
import type { CommandAction, CommandItem } from './model';

type LucideComponent = ComponentType<{ size?: number; className?: string }>;

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
} as const;

export const iconFor = Object.fromEntries(
  Object.entries(curatedIconAliases).map(([name, lucideName]) => [
    name,
    LucideIcons[lucideName] as LucideComponent,
  ]),
) as Record<keyof typeof curatedIconAliases, LucideComponent>;

export type CommandIconName = string;

function pascalCaseIconName(name: string) {
  return name
    .replace(/Icon$/, '')
    .split(/[^a-zA-Z0-9]+/)
    .filter(Boolean)
    .map((part) => `${part.slice(0, 1).toUpperCase()}${part.slice(1)}`)
    .join('');
}

function isRenderableLucideIcon(value: unknown): value is LucideComponent {
  if (!(typeof value === 'object' || typeof value === 'function')) return false;
  // lucide-react also exports helpers such as `Icon`, `icons`, and
  // `createLucideIcon`. Some of them are objects/functions but are not
  // complete icon components; rendering them crashes inside lucide-react.
  // Real generated icon components have a displayName (for example Trash2).
  return typeof (value as { displayName?: unknown }).displayName === 'string';
}

function lucideIcon(
  name: unknown,
  fallback: keyof typeof curatedIconAliases = 'sparkles',
) {
  const requested = String(name || '').trim();
  const alias =
    curatedIconAliases[requested as keyof typeof curatedIconAliases];
  const pascalName = pascalCaseIconName(requested);
  const candidates = [
    alias,
    requested,
    pascalName,
    pascalName ? `${pascalName}Icon` : '',
  ].filter(Boolean);
  for (const candidate of candidates) {
    const Icon = (LucideIcons as Record<string, unknown>)[candidate];
    if (isRenderableLucideIcon(Icon)) return Icon;
  }
  return iconFor[fallback];
}

export function iconForAction(action: CommandAction) {
  if (typeof action.icon === 'string' && action.icon) {
    const Icon = lucideIcon(action.icon);
    return <Icon size={18} />;
  }
  if (action.style === 'destructive') return <LucideIcons.Trash2 size={18} />;
  if (
    action.type === 'copyText' ||
    action.type === 'copyImage' ||
    action.type === 'pasteText'
  )
    return <LucideIcons.Clipboard size={18} />;
  if (action.type === 'trash' || action.type === 'removeClipboardHistory')
    return <LucideIcons.Trash2 size={18} />;
  if (
    action.type === 'revealPath' ||
    action.type === 'openPath' ||
    action.type === 'quickLook' ||
    action.type === 'openWith'
  )
    return <LucideIcons.Folder size={18} />;
  if (action.type === 'nativeAction') return <LucideIcons.Sparkles size={18} />;
  return <LucideIcons.Globe size={18} />;
}

function imageSource(image: CommandItem['image']) {
  if (!image) return '';
  if (typeof image === 'string') return image;
  return image.dark || image.src || image.light || image.fallback || '';
}

export function iconForItem(
  item: CommandItem,
  fallback: CommandIconName = 'sparkles',
) {
  const Icon = lucideIcon(
    item.icon,
    fallback as keyof typeof curatedIconAliases,
  );
  const image = imageSource(item.image);
  return image ? (
    <span className="thumbnailIcon">
      <img src={image} alt="" />
    </span>
  ) : (
    <Icon size={18} />
  );
}

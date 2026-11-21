import type { PaletteItem } from './palette-types';

export const AUTOMATE_ID = 'fallback:automate';
export const WEB_SEARCH_ID = 'fallback:web';
/** The 2.x default search engine (src/native/internal/ext/builtins/websearch). */
const WEB_SEARCH_URL = 'https://www.google.com/search?q=';
/**
 * Where the query fallbacks land among the results. 2.x scores Automate at 40, which puts it under the strong
 * matches and above the weak fuzzy ones; the landing scorer has no such band, so a fixed slot stands in for it.
 */
const FALLBACK_SLOT = 7;

export interface MenuAction {
  id: string;
  title: string;
  /** Keycap label shown at the row's end; the primary action shows ↵ instead. */
  shortcut?: string;
  primary?: boolean;
}

function automateRow(query: string): PaletteItem {
  return {
    id: AUTOMATE_ID,
    title: `Automate "${query}"`,
    subtitle: 'Build a reusable command with AI',
    icon: 'zap',
    aliases: [],
    kind: 'command',
    primary: 'Automate',
  };
}

function webSearchRow(query: string): PaletteItem {
  return {
    id: WEB_SEARCH_ID,
    title: `Search the web for "${query}"`,
    subtitle: 'Search instead',
    icon: 'search',
    aliases: [],
    kind: 'command',
    primary: 'Search the Web',
    url: `${WEB_SEARCH_URL}${encodeURIComponent(query)}`,
  };
}

/** Adds the 2.x query fallbacks, Automate and the web search, to the results of a non-empty query. */
export function withQueryFallbacks(results: PaletteItem[], query: string): PaletteItem[] {
  const text = query.trim().replace(/\s+/g, ' ');
  if (!text) return results;
  const slot = Math.min(results.length, FALLBACK_SLOT);
  return [...results.slice(0, slot), automateRow(text), webSearchRow(text), ...results.slice(slot)];
}

export function isAutomate(item: PaletteItem) {
  return item.id === AUTOMATE_ID;
}

/** The title of an item's Enter action, as the footer and the action menu show it. */
export function primaryTitle(item: PaletteItem): string {
  if (item.primary) return item.primary;
  switch (item.kind) {
    case 'app':
    case 'file':
      return `Open ${item.title}`;
    case 'clipboard':
      return 'Paste Text';
    default:
      return item.title;
  }
}

function secondaryActions(item: PaletteItem): MenuAction[] {
  if (item.id === AUTOMATE_ID || item.id === WEB_SEARCH_ID) return [];
  switch (item.kind) {
    case 'app':
      return [
        { id: 'uninstall', title: `Uninstall ${item.title}…` },
        { id: 'reveal', title: 'Show in File Manager', shortcut: '⌘↵' },
        { id: 'shortcut', title: 'Set keyboard shortcut' },
        { id: 'alias', title: 'Set alias' },
      ];
    case 'file':
      return [
        { id: 'preview', title: 'Preview' },
        { id: 'reveal', title: 'Show in File Manager', shortcut: '⌘↵' },
      ];
    case 'clipboard':
      return [
        { id: 'copy', title: 'Copy Text' },
        { id: 'preview', title: 'Preview Contents' },
        { id: 'remove', title: 'Remove Item' },
      ];
    default:
      return [
        { id: 'shortcut', title: 'Set keyboard shortcut' },
        { id: 'alias', title: 'Set alias' },
      ];
  }
}

/** The rows of an item's ⌘K action menu: its Enter action first, as in 2.x. */
export function menuActions(item: PaletteItem): MenuAction[] {
  return [{ id: 'primary', title: primaryTitle(item), primary: true }, ...secondaryActions(item)];
}

/** The ⌘K actions of the AI chat and builder pages. */
export const CHAT_ACTIONS: MenuAction[] = [
  { id: 'primary', title: 'Send', primary: true },
  { id: 'new-chat', title: 'New Chat' },
  { id: 'switch-model', title: 'Switch to Smart' },
];

// biome-ignore-all lint: Menu bar descriptors are authored by extensions and normalized defensively.
import {
  DEFAULT_MENUBAR_ICON,
  isMenuBarIconName,
  type MenuBarIconName,
} from '../../shared/menubar-icons';

const MAX_MENU_BARS_PER_EXTENSION = 3;
const MAX_SECTIONS = 8;
const MAX_ROWS_PER_SECTION = 40;
const MAX_SUBMENU_DEPTH = 2;
const MAX_ROW_TEXT = 120;
const MAX_STATUS_TITLE = 24;

export type NormalizedMenuBarRow = {
  id: string;
  title: string;
  subtitle?: string;
  icon?: MenuBarIconName;
  checked?: boolean;
  disabled?: boolean;
  shortcut?: string;
  action?: unknown;
  items?: NormalizedMenuBarRow[];
};

export type NormalizedMenuBarSection = {
  title?: string;
  items: NormalizedMenuBarRow[];
};

export type NormalizedMenuBar = {
  id: string;
  icon: MenuBarIconName;
  title?: string;
  tooltip: string;
  sections: NormalizedMenuBarSection[];
};

export type MenuBarNormalizationIssue = { menuBarId?: string; message: string };

export function extensionMenuBarRenderer(extension: any) {
  const contribution = extension?.menuBar;
  if (typeof contribution === 'function') return contribution;
  return typeof contribution?.render === 'function'
    ? contribution.render
    : null;
}

export function extensionMenuBarTriggers(extension: any): unknown[] {
  const triggers = extension?.menuBar?.triggers;
  return Array.isArray(triggers) ? triggers : [];
}

function boundedText(value: unknown, max: number) {
  if (typeof value !== 'string') return undefined;
  const text = value.replace(/\s+/g, ' ').trim();
  if (!text) return undefined;
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
}

function optionalIcon(value: unknown, issues: MenuBarNormalizationIssue[]) {
  if (value === undefined) return undefined;
  if (isMenuBarIconName(value)) return value;
  issues.push({ message: `Unknown menu bar icon "${String(value)}"` });
  return undefined;
}

function normalizeRow(
  row: any,
  depth: number,
  issues: MenuBarNormalizationIssue[],
): NormalizedMenuBarRow | null {
  const id = boundedText(row?.id, MAX_ROW_TEXT);
  const title = boundedText(row?.title, MAX_ROW_TEXT);
  if (!(id && title)) {
    issues.push({ message: 'Menu bar rows need an id and a title' });
    return null;
  }
  const items =
    depth < MAX_SUBMENU_DEPTH && Array.isArray(row.items)
      ? normalizeRows(row.items, depth + 1, issues)
      : undefined;
  return {
    id,
    title,
    ...(boundedText(row.subtitle, MAX_ROW_TEXT)
      ? { subtitle: boundedText(row.subtitle, MAX_ROW_TEXT) }
      : {}),
    ...(optionalIcon(row.icon, issues) ? { icon: row.icon } : {}),
    ...(typeof row.checked === 'boolean' ? { checked: row.checked } : {}),
    ...(row.disabled === true ? { disabled: true } : {}),
    ...(boundedText(row.shortcut, 64) ? { shortcut: row.shortcut } : {}),
    ...(items?.length ? { items } : row.action ? { action: row.action } : {}),
  };
}

function normalizeRows(
  rows: unknown[],
  depth: number,
  issues: MenuBarNormalizationIssue[],
) {
  if (rows.length > MAX_ROWS_PER_SECTION)
    issues.push({
      message: `Menu bar sections show at most ${MAX_ROWS_PER_SECTION} rows`,
    });
  return rows
    .slice(0, MAX_ROWS_PER_SECTION)
    .map((row) => normalizeRow(row, depth, issues))
    .filter((row): row is NormalizedMenuBarRow => Boolean(row));
}

function sectionsFromDescriptor(descriptor: any) {
  if (Array.isArray(descriptor?.sections)) return descriptor.sections;
  return Array.isArray(descriptor?.items) ? [{ items: descriptor.items }] : [];
}

function normalizeSections(
  descriptor: any,
  issues: MenuBarNormalizationIssue[],
): NormalizedMenuBarSection[] {
  return sectionsFromDescriptor(descriptor)
    .slice(0, MAX_SECTIONS)
    .map((section: any) => ({
      ...(boundedText(section?.title, MAX_ROW_TEXT)
        ? { title: boundedText(section.title, MAX_ROW_TEXT) }
        : {}),
      items: Array.isArray(section?.items)
        ? normalizeRows(section.items, 1, issues)
        : [],
    }))
    .filter((section) => section.items.length > 0);
}

function normalizeMenuBar(
  extension: any,
  descriptor: any,
  index: number,
  issues: MenuBarNormalizationIssue[],
): NormalizedMenuBar | null {
  if (!descriptor || typeof descriptor !== 'object') return null;
  const id = boundedText(descriptor.id, 64) || (index === 0 ? 'main' : '');
  if (!id) {
    issues.push({ message: 'Extra menu bar items need a stable id' });
    return null;
  }
  const scopedIssues: MenuBarNormalizationIssue[] = [];
  const icon =
    optionalIcon(descriptor.icon, scopedIssues) || DEFAULT_MENUBAR_ICON;
  const title = boundedText(descriptor.title, MAX_STATUS_TITLE);
  const normalized = {
    id,
    icon,
    ...(title ? { title } : {}),
    tooltip:
      boundedText(descriptor.tooltip, MAX_ROW_TEXT) ||
      boundedText(extension?.title, MAX_ROW_TEXT) ||
      String(extension?.id || 'Nevermind'),
    sections: normalizeSections(descriptor, scopedIssues),
  };
  for (const issue of scopedIssues) issues.push({ ...issue, menuBarId: id });
  return normalized;
}

function descriptorsFromResult(result: unknown) {
  if (!result) return [];
  return Array.isArray(result) ? result : [result];
}

export function normalizeExtensionMenuBars(extension: any, result: unknown) {
  const issues: MenuBarNormalizationIssue[] = [];
  const descriptors = descriptorsFromResult(result);
  if (descriptors.length > MAX_MENU_BARS_PER_EXTENSION)
    issues.push({
      message: `Extensions show at most ${MAX_MENU_BARS_PER_EXTENSION} menu bar items`,
    });
  const seen = new Set<string>();
  const menuBars = descriptors
    .slice(0, MAX_MENU_BARS_PER_EXTENSION)
    .map((descriptor, index) =>
      normalizeMenuBar(extension, descriptor, index, issues),
    )
    .filter((menuBar): menuBar is NormalizedMenuBar => {
      if (!menuBar || seen.has(menuBar.id)) return false;
      seen.add(menuBar.id);
      return true;
    });
  return { menuBars, issues };
}

export function menuBarStatusItemId(extensionId: string, menuBarId: string) {
  return `${extensionId}:${menuBarId}`;
}

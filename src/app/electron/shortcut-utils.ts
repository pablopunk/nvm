// biome-ignore-all lint/style/useNamingConvention: Electron accelerator tokens are canonical title-case external values.
import { isReservedPaletteAccelerator } from './os';

const SHORTCUT_SYMBOLS: Record<string, string> = {
  Command: '⌘',
  Cmd: '⌘',
  Control: '⌃',
  Ctrl: '⌃',
  Alt: '⌥',
  Option: '⌥',
  Shift: '⇧',
  Enter: '↵',
  Return: '↵',
  Escape: 'Esc',
  Tab: 'Tab',
};

const SHORTCUT_ALIASES = new Map([
  ['cmd', 'Command'],
  ['command', 'Command'],
  ['⌘', 'Command'],
  ['ctrl', 'Control'],
  ['control', 'Control'],
  ['^', 'Control'],
  ['option', 'Alt'],
  ['opt', 'Alt'],
  ['alt', 'Alt'],
  ['⌥', 'Alt'],
  ['shift', 'Shift'],
  ['⇧', 'Shift'],
  ['enter', 'Enter'],
  ['return', 'Enter'],
  ['↵', 'Enter'],
  ['esc', 'Escape'],
  ['escape', 'Escape'],
  ['space', 'Space'],
]);

function normalizeAcceleratorPart(part: string) {
  return (
    SHORTCUT_ALIASES.get(part.toLowerCase()) ||
    (part.length === 1 ? part.toUpperCase() : part)
  );
}

export function formatShortcut(
  accelerator: unknown,
  processPlatform: NodeJS.Platform = process.platform,
) {
  const parts = normalizeAccelerator(accelerator).split('+').filter(Boolean);
  if (processPlatform !== 'darwin') {
    return parts.join('+');
  }
  return parts.map((part) => SHORTCUT_SYMBOLS[part] || part).join('');
}

export function normalizeAccelerator(value: unknown) {
  return String(value || '')
    .split('+')
    .map((part) => part.trim())
    .filter(Boolean)
    .map(normalizeAcceleratorPart)
    .join('+');
}

export function isSpotlightAccelerator(accelerator: unknown) {
  return isReservedPaletteAccelerator(normalizeAccelerator(accelerator));
}

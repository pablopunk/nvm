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

export function formatShortcut(
  accelerator: unknown,
  processPlatform: NodeJS.Platform = process.platform,
) {
  const parts = normalizeAccelerator(accelerator).split('+').filter(Boolean);
  if (processPlatform !== 'darwin') return parts.join('+');
  return parts.map((part) => SHORTCUT_SYMBOLS[part] || part).join('');
}

export function normalizeAccelerator(value: unknown) {
  return String(value || '')
    .split('+')
    .map((part) => part.trim())
    .filter(Boolean)
    .map((part) => {
      const normalized = part.toLowerCase();
      if (['cmd', 'command', '⌘'].includes(normalized)) return 'Command';
      if (['ctrl', 'control', '^'].includes(normalized)) return 'Control';
      if (['option', 'opt', 'alt', '⌥'].includes(normalized)) return 'Alt';
      if (['shift', '⇧'].includes(normalized)) return 'Shift';
      if (['enter', 'return', '↵'].includes(normalized)) return 'Enter';
      if (['esc', 'escape'].includes(normalized)) return 'Escape';
      if (normalized === 'space') return 'Space';
      return part.length === 1 ? part.toUpperCase() : part;
    })
    .join('+');
}

export function isSpotlightAccelerator(accelerator: unknown) {
  return isReservedPaletteAccelerator(normalizeAccelerator(accelerator));
}

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
}

export function formatShortcut(accelerator: unknown) {
  return String(accelerator || '').split('+').map((part) => SHORTCUT_SYMBOLS[part] || part).join('')
}

export function normalizeAccelerator(value: unknown) {
  return String(value || '')
    .split('+')
    .map((part) => part.trim())
    .filter(Boolean)
    .map((part) => {
      const normalized = part.toLowerCase()
      if (['cmd', 'command', '⌘'].includes(normalized)) return 'Command'
      if (['ctrl', 'control', '^'].includes(normalized)) return 'Control'
      if (['option', 'opt', 'alt', '⌥'].includes(normalized)) return 'Alt'
      if (['shift', '⇧'].includes(normalized)) return 'Shift'
      if (['enter', 'return', '↵'].includes(normalized)) return 'Enter'
      if (['esc', 'escape'].includes(normalized)) return 'Escape'
      if (normalized === 'space') return 'Space'
      return part.length === 1 ? part.toUpperCase() : part
    })
    .join('+')
}

export function isSpotlightAccelerator(accelerator: unknown) {
  if (process.platform !== 'darwin') return false
  return normalizeAccelerator(accelerator) === 'Command+Space'
}

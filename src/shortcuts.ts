export function keyNameForShortcut(event: Pick<KeyboardEvent, 'key' | 'code'>) {
  const key = event.key
  if (!key || ['Meta', 'Control', 'Alt', 'Shift'].includes(key)) return ''
  if (key === ' ') return 'Space'
  if (key === 'ArrowUp') return 'Up'
  if (key === 'ArrowDown') return 'Down'
  if (key === 'ArrowLeft') return 'Left'
  if (key === 'ArrowRight') return 'Right'
  if (/^Digit\d$/.test(event.code)) return event.code.slice('Digit'.length)
  if (/^Key[A-Z]$/.test(event.code)) return event.code.slice('Key'.length)
  if (key.length === 1) return key.toUpperCase()
  return key[0].toUpperCase() + key.slice(1)
}

export function acceleratorFromKeyboardEvent(event: Pick<KeyboardEvent, 'key' | 'code' | 'metaKey' | 'ctrlKey' | 'altKey' | 'shiftKey'>) {
  const key = keyNameForShortcut(event)
  if (!key) return ''
  const parts = []
  if (event.metaKey) parts.push('Command')
  if (event.ctrlKey) parts.push('Control')
  if (event.altKey) parts.push('Alt')
  if (event.shiftKey) parts.push('Shift')
  parts.push(key)
  if (parts.length < 2 && !key.startsWith('F')) return ''
  return parts.join('+')
}

export function normalizedShortcut(value?: string) {
  return String(value || '').replace(/\s+/g, '').toLowerCase()
}

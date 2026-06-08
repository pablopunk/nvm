const SAFE_EXTERNAL_PROTOCOLS = new Set(['http:', 'https:', 'mailto:'])

export function safeExternalUrl(raw: unknown) {
  const value = String(raw || '').trim()
  if (!value) return null
  try {
    const url = new URL(value)
    if (!SAFE_EXTERNAL_PROTOCOLS.has(url.protocol)) return null
    if ((url.protocol === 'http:' || url.protocol === 'https:') && !url.hostname) return null
    return url.href
  } catch {
    return null
  }
}

async function electronShell() {
  const electron = await import('electron') as typeof import('electron') & { default?: typeof import('electron') }
  return electron.shell || electron.default?.shell
}

export async function openExternalUrl(raw: unknown) {
  const url = safeExternalUrl(raw)
  if (!url) return false
  try {
    const shell = await electronShell()
    if (!shell) return false
    await shell.openExternal(url)
    return true
  } catch {
    return false
  }
}

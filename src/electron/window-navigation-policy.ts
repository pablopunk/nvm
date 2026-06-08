import { fileURLToPath } from 'node:url'
import { openExternalUrl } from './url-utils'

type WindowOpenHandlerDetails = { url: string }
type NavigationEvent = { preventDefault: () => void }
type WebContentsWithNavigationPolicy = {
  setWindowOpenHandler: (handler: (details: WindowOpenHandlerDetails) => { action: 'allow' | 'deny' }) => void
  on: (event: 'will-navigate', listener: (event: NavigationEvent, url: string) => void) => void
}

type WindowWithNavigationPolicy = { webContents: WebContentsWithNavigationPolicy }

function isTrustedFilePage(url: string, rendererIndexPath?: string) {
  if (!rendererIndexPath) return false
  try {
    const parsed = new URL(url)
    return parsed.protocol === 'file:' && fileURLToPath(parsed) === rendererIndexPath
  } catch {
    return false
  }
}

function isTrustedDevRendererPage(url: string, rendererUrl: string, requiresExtensionWindowId?: string) {
  if (!rendererUrl) return false
  try {
    const parsed = new URL(url)
    const expected = new URL(rendererUrl)
    if (parsed.origin !== expected.origin || parsed.pathname !== expected.pathname) return false
    if (requiresExtensionWindowId === undefined) return true
    return parsed.searchParams.get('extensionWindowId') === requiresExtensionWindowId
  } catch {
    return false
  }
}

export function isTrustedAppPage(url: string, isDev: boolean, rendererUrl = process.env.ELECTRON_RENDERER_URL || '', rendererIndexPath?: string) {
  return isTrustedFilePage(url, rendererIndexPath) || (isDev && isTrustedDevRendererPage(url, rendererUrl))
}

export function isTrustedExtensionWindowPage(url: string, id: string, isDev: boolean, rendererUrl?: string, rendererIndexPath?: string) {
  if (isDev && isTrustedDevRendererPage(url, rendererUrl || '', id)) return true
  if (!isTrustedFilePage(url, rendererIndexPath)) return false
  try {
    return new URL(url).searchParams.get('extensionWindowId') === id
  } catch {
    return false
  }
}

export function installExternalNavigationPolicy(
  win: WindowWithNavigationPolicy,
  isTrustedNavigation: (url: string) => boolean,
  openExternal: (url: string) => Promise<boolean> = openExternalUrl,
) {
  win.webContents.setWindowOpenHandler(({ url }) => {
    void openExternal(url).catch(() => false)
    return { action: 'deny' }
  })
  win.webContents.on('will-navigate', (event, url) => {
    if (isTrustedNavigation(url)) return
    event.preventDefault()
    void openExternal(url).catch(() => false)
  })
}

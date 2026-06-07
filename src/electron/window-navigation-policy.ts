import { openExternalUrl } from './url-utils'

type WindowOpenHandlerDetails = { url: string }
type NavigationEvent = { preventDefault: () => void }
type WebContentsWithNavigationPolicy = {
  setWindowOpenHandler: (handler: (details: WindowOpenHandlerDetails) => { action: 'allow' | 'deny' }) => void
  on: (event: 'will-navigate', listener: (event: NavigationEvent, url: string) => void) => void
}

type WindowWithNavigationPolicy = { webContents: WebContentsWithNavigationPolicy }

export function isTrustedAppPage(url: string, isDev: boolean, rendererUrl = process.env.ELECTRON_RENDERER_URL || '') {
  return url.startsWith('file:') || (isDev && Boolean(rendererUrl) && url.startsWith(rendererUrl))
}

export function isTrustedExtensionWindowPage(url: string, id: string, isDev: boolean, rendererUrl?: string) {
  const expectedDevUrl = rendererUrl ? `${rendererUrl}?extensionWindowId=${encodeURIComponent(id)}` : ''
  return url.startsWith('file:') || (isDev && Boolean(expectedDevUrl) && url.startsWith(expectedDevUrl))
}

export function installExternalNavigationPolicy(
  win: WindowWithNavigationPolicy,
  isTrustedNavigation: (url: string) => boolean,
  openExternal: (url: string) => Promise<boolean> = openExternalUrl,
) {
  win.webContents.setWindowOpenHandler(({ url }) => {
    void openExternal(url)
    return { action: 'deny' }
  })
  win.webContents.on('will-navigate', (event, url) => {
    if (isTrustedNavigation(url)) return
    event.preventDefault()
    void openExternal(url)
  })
}

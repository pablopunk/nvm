import { app } from 'electron'
import { supportsAutoUpdates } from './os'

type UpdateInfo = { version?: string }
type AutoUpdaterLike = {
  autoDownload: boolean
  autoInstallOnAppQuit: boolean
  checkForUpdates: () => Promise<{ updateInfo?: UpdateInfo } | null | undefined>
  downloadUpdate: () => Promise<unknown>
  quitAndInstall: () => void
  on: (event: string, listener: (...args: any[]) => void) => void
}

type UpdateStatus = 'idle' | 'unsupported' | 'checking' | 'available' | 'downloading' | 'downloaded' | 'error'

type UpdateState = {
  checkInFlight: boolean
  downloadInFlight: boolean
  status: UpdateStatus
  availableInfo: UpdateInfo | null
  downloadedInfo: UpdateInfo | null
  errorMessage: string
}

const AUTO_UPDATE_STARTUP_DELAY_MS = 15_000
const AUTO_UPDATE_POLL_INTERVAL_MS = 4 * 60 * 60 * 1000

export function createUpdateManager(autoUpdater: AutoUpdaterLike) {
  let startupTimer: NodeJS.Timeout | null = null
  let pollTimer: NodeJS.Timeout | null = null
  const state: UpdateState = {
    checkInFlight: false,
    downloadInFlight: false,
    status: 'idle',
    availableInfo: null,
    downloadedInfo: null,
    errorMessage: '',
  }

  function canUseAutoUpdates() {
    return app.isPackaged && supportsAutoUpdates()
  }

  async function checkForUpdates(trigger = 'manual', options: { download?: boolean } = {}) {
    if (!canUseAutoUpdates()) {
      state.status = 'unsupported'
      return null
    }
    if (state.downloadedInfo) return state.downloadedInfo
    if (state.checkInFlight || state.downloadInFlight) return state.availableInfo || state.downloadedInfo
    state.checkInFlight = true
    state.status = 'checking'
    state.errorMessage = ''
    try {
      console.info(`[nvm-updater] checking for updates (${trigger})`)
      const result = await autoUpdater.checkForUpdates()
      if (options.download && result?.updateInfo && !state.downloadedInfo) await downloadAvailableUpdate(result.updateInfo)
      return result?.updateInfo || null
    } catch (error) {
      state.status = 'error'
      state.errorMessage = error instanceof Error ? error.message : String(error)
      console.error('[nvm-updater] update check failed', error)
      return null
    } finally {
      state.checkInFlight = false
      if (state.status === 'checking') state.status = state.availableInfo ? 'available' : 'idle'
    }
  }

  async function downloadAvailableUpdate(info = state.availableInfo) {
    if (!canUseAutoUpdates() || state.downloadInFlight || state.downloadedInfo) return
    state.availableInfo = info || state.availableInfo
    state.downloadInFlight = true
    state.status = 'downloading'
    state.errorMessage = ''
    try {
      await autoUpdater.downloadUpdate()
    } catch (error) {
      state.status = 'error'
      state.errorMessage = error instanceof Error ? error.message : String(error)
      console.error('[nvm-updater] update download failed', error)
    } finally {
      state.downloadInFlight = false
      if (state.status === 'downloading') state.status = state.downloadedInfo ? 'downloaded' : state.availableInfo ? 'available' : 'idle'
    }
  }

  function clearTimers() {
    if (startupTimer) {
      clearTimeout(startupTimer)
      startupTimer = null
    }
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  function configure() {
    if (!canUseAutoUpdates()) {
      console.info('[nvm-updater] disabled (development build or unsupported platform)')
      return
    }

    autoUpdater.autoDownload = false
    autoUpdater.autoInstallOnAppQuit = false

    autoUpdater.on('checking-for-update', () => {
      state.status = 'checking'
      console.info('[nvm-updater] checking for update')
    })

    autoUpdater.on('update-available', (info: UpdateInfo) => {
      state.availableInfo = info
      state.status = 'available'
      console.info(`[nvm-updater] update available ${info.version}`)
    })

    autoUpdater.on('update-not-available', () => {
      state.availableInfo = null
      state.downloadedInfo = null
      state.status = 'idle'
      console.info('[nvm-updater] no updates available')
    })

    autoUpdater.on('download-progress', (progress: { percent: number }) => {
      console.info(`[nvm-updater] download progress ${Math.floor(progress.percent)}%`)
    })

    autoUpdater.on('update-downloaded', (info: UpdateInfo) => {
      state.downloadedInfo = info
      state.availableInfo = info
      state.status = 'downloaded'
      console.info(`[nvm-updater] update downloaded ${info.version}`)
    })

    autoUpdater.on('error', (error: Error) => {
      state.status = 'error'
      state.errorMessage = error?.message || String(error)
      console.error('[nvm-updater] updater error', error)
    })

    startupTimer = setTimeout(() => {
      startupTimer = null
      void checkForUpdates('startup')
    }, AUTO_UPDATE_STARTUP_DELAY_MS)
    startupTimer.unref?.()

    pollTimer = setInterval(() => {
      void checkForUpdates('poll')
    }, AUTO_UPDATE_POLL_INTERVAL_MS)
    pollTimer.unref?.()
  }

  function quitAndInstall() {
    autoUpdater.quitAndInstall()
  }

  return { state, canUseAutoUpdates, checkForUpdates, downloadAvailableUpdate, clearTimers, configure, quitAndInstall }
}

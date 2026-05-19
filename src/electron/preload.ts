import { contextBridge, ipcRenderer, type IpcRendererEvent } from 'electron'
import type { NevermindApi } from '../preload-api'

const api: NevermindApi = {
  search: (query, options) => ipcRenderer.invoke('actions:search', query, options),
  execute: (action) => ipcRenderer.invoke('actions:execute', action),
  runViewAction: (action) => ipcRenderer.invoke('view-action:execute', action),
  startFileDrag: (filePath) => ipcRenderer.send('drag:file', filePath),
  sendAiMessage: (message, chatId) => ipcRenderer.invoke('ai:chat:send', message, chatId),
  abortAiChat: (chatId) => ipcRenderer.invoke('ai:chat:abort', chatId),
  resetAiChat: (chatId) => ipcRenderer.invoke('ai:chat:reset', chatId),
  setAlias: (action, alias) => ipcRenderer.invoke('actions:set-alias', action, alias),
  removeAlias: (action, alias) => ipcRenderer.invoke('actions:remove-alias', action, alias),
  setShortcut: (action, shortcut) => ipcRenderer.invoke('actions:set-shortcut', action, shortcut),
  setPaletteHotkey: (accelerator) => ipcRenderer.invoke('palette:set-hotkey', accelerator),
  openSystemKeyboardSettings: () => ipcRenderer.invoke('system:open-keyboard-settings'),
  getShortcuts: () => ipcRenderer.invoke('actions:get-shortcuts'),
  removeShortcut: (actionId) => ipcRenderer.invoke('actions:remove-shortcut', actionId),
  suspendShortcuts: () => ipcRenderer.invoke('actions:suspend-shortcuts'),
  resumeShortcuts: () => ipcRenderer.invoke('actions:resume-shortcuts'),
  setOverride: (action, instruction) => ipcRenderer.invoke('actions:set-override', action, instruction),
  clearOverride: (action) => ipcRenderer.invoke('actions:clear-override', action),
  duplicateCreatedAction: (action) => ipcRenderer.invoke('actions:duplicate-created', action),
  removeCreatedAction: (action) => ipcRenderer.invoke('actions:remove-created', action),
  getAppIcon: (appPath) => ipcRenderer.invoke('apps:icon', appPath),
  setPaletteMode: (mode) => ipcRenderer.invoke('palette:set-mode', mode),
  hide: () => ipcRenderer.invoke('palette:hide'),
  shortcutReady: () => ipcRenderer.invoke('palette:shortcut-ready'),
  log: (level, message, data) => ipcRenderer.invoke('logs:write', level, message, data),
  onShown: (callback) => {
    const listener = () => callback()
    ipcRenderer.on('palette:shown', listener)
    return () => ipcRenderer.removeListener('palette:shown', listener)
  },
  onShortcutShown: (callback) => {
    const listener = () => callback()
    ipcRenderer.on('palette:shortcut-show', listener)
    return () => ipcRenderer.removeListener('palette:shortcut-show', listener)
  },
  onHidden: (callback) => {
    const listener = () => callback()
    ipcRenderer.on('palette:hidden', listener)
    return () => ipcRenderer.removeListener('palette:hidden', listener)
  },
  onAppsIndexed: (callback) => {
    const listener = (_event: IpcRendererEvent, count: number) => callback(count)
    ipcRenderer.on('apps:indexed', listener)
    return () => ipcRenderer.removeListener('apps:indexed', listener)
  },
  onClipboardChanged: (callback) => {
    const listener = () => callback()
    ipcRenderer.on('clipboard:changed', listener)
    return () => ipcRenderer.removeListener('clipboard:changed', listener)
  },
  onRootItemsChanged: (callback) => {
    const listener = () => callback()
    ipcRenderer.on('root-items:changed', listener)
    return () => ipcRenderer.removeListener('root-items:changed', listener)
  },
  onOpenActionView: (callback) => {
    const listener = (_event: IpcRendererEvent, payload?: Parameters<NevermindApi['onOpenActionView']>[0] extends (payload?: infer Payload) => void ? Payload : never) => callback(payload)
    ipcRenderer.on('action:view-open', listener)
    return () => ipcRenderer.removeListener('action:view-open', listener)
  },
  onAiChatEvent: (callback) => {
    const listener = (_event: IpcRendererEvent, payload: Parameters<NevermindApi['onAiChatEvent']>[0] extends (event: infer Event) => void ? Event : never) => callback(payload)
    ipcRenderer.on('ai:chat:event', listener)
    return () => ipcRenderer.removeListener('ai:chat:event', listener)
  },
}

contextBridge.exposeInMainWorld('nvm', api)

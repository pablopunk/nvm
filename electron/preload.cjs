const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('nvm', {
  search: (query, options) => ipcRenderer.invoke('actions:search', query, options),
  execute: (action) => ipcRenderer.invoke('actions:execute', action),
  runViewAction: (action) => ipcRenderer.invoke('view-action:execute', action),
  startFileDrag: (filePath) => ipcRenderer.send('drag:file', filePath),
  sendAiMessage: (message, chatId) => ipcRenderer.invoke('ai:chat:send', message, chatId),
  abortAiChat: (chatId) => ipcRenderer.invoke('ai:chat:abort', chatId),
  resetAiChat: (chatId) => ipcRenderer.invoke('ai:chat:reset', chatId),
  setAlias: (action, alias) => ipcRenderer.invoke('actions:set-alias', action, alias),
  setShortcut: (action, shortcut) => ipcRenderer.invoke('actions:set-shortcut', action, shortcut),
  setOverride: (action, instruction) => ipcRenderer.invoke('actions:set-override', action, instruction),
  clearOverride: (action) => ipcRenderer.invoke('actions:clear-override', action),
  removeCreatedAction: (action) => ipcRenderer.invoke('actions:remove-created', action),
  getAppIcon: (appPath) => ipcRenderer.invoke('apps:icon', appPath),
  setPaletteMode: (mode) => ipcRenderer.invoke('palette:set-mode', mode),
  hide: () => ipcRenderer.invoke('palette:hide'),
  onShown: (callback) => {
    const listener = () => callback()
    ipcRenderer.on('palette:shown', listener)
    return () => ipcRenderer.removeListener('palette:shown', listener)
  },
  onHidden: (callback) => {
    const listener = () => callback()
    ipcRenderer.on('palette:hidden', listener)
    return () => ipcRenderer.removeListener('palette:hidden', listener)
  },
  onAppsIndexed: (callback) => {
    const listener = (_event, count) => callback(count)
    ipcRenderer.on('apps:indexed', listener)
    return () => ipcRenderer.removeListener('apps:indexed', listener)
  },
  onClipboardChanged: (callback) => {
    const listener = () => callback()
    ipcRenderer.on('clipboard:changed', listener)
    return () => ipcRenderer.removeListener('clipboard:changed', listener)
  },
  onAiChatEvent: (callback) => {
    const listener = (_event, payload) => callback(payload)
    ipcRenderer.on('ai:chat:event', listener)
    return () => ipcRenderer.removeListener('ai:chat:event', listener)
  },
})

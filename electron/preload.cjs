const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('mvm', {
  search: (query, options) => ipcRenderer.invoke('actions:search', query, options),
  execute: (action) => ipcRenderer.invoke('actions:execute', action),
  setAlias: (action, alias) => ipcRenderer.invoke('actions:set-alias', action, alias),
  setShortcut: (action, shortcut) => ipcRenderer.invoke('actions:set-shortcut', action, shortcut),
  setOverride: (action, instruction) => ipcRenderer.invoke('actions:set-override', action, instruction),
  clearOverride: (action) => ipcRenderer.invoke('actions:clear-override', action),
  getAppIcon: (appPath) => ipcRenderer.invoke('apps:icon', appPath),
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
})

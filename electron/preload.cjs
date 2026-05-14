const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('mvm', {
  search: (query) => ipcRenderer.invoke('actions:search', query),
  execute: (action) => ipcRenderer.invoke('actions:execute', action),
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
})

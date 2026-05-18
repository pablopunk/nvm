import os from 'node:os'
import path from 'node:path'
import { settingsTitle } from './os'

export function builtInActions({ version }: { version: string; platform?: NodeJS.Platform }) {
  return [
    {
      id: 'builtin:check-for-updates',
      kind: 'check-for-updates',
      title: 'Check for Updates',
      subtitle: `Current version: ${version}`,
      icon: 'restart',
      score: 23,
    },
    { id: 'builtin:lock-screen', kind: 'builtin', builtin: 'lock-screen', title: 'Lock Screen', subtitle: 'Secure this computer', icon: 'lock', score: 22 },
    { id: 'builtin:sleep', kind: 'builtin', builtin: 'sleep', title: 'Sleep', subtitle: 'Put this computer to sleep', icon: 'moon', score: 21 },
    { id: 'builtin:restart', kind: 'builtin', builtin: 'restart', title: 'Restart Computer', subtitle: 'Restart this computer', icon: 'restart', score: 20 },
    { id: 'builtin:settings', kind: 'builtin', builtin: 'settings', title: settingsTitle(), subtitle: 'Open system preferences', icon: 'settings', score: 19 },
    { id: 'builtin:downloads', kind: 'builtin', builtin: 'open-path', targetPath: path.join(os.homedir(), 'Downloads'), title: 'Open Downloads', subtitle: '~/Downloads', icon: 'folder', score: 18 },
    { id: 'builtin:documents', kind: 'builtin', builtin: 'open-path', targetPath: path.join(os.homedir(), 'Documents'), title: 'Open Documents', subtitle: '~/Documents', icon: 'folder', score: 17 },
    { id: 'builtin:desktop', kind: 'builtin', builtin: 'open-path', targetPath: path.join(os.homedir(), 'Desktop'), title: 'Open Desktop', subtitle: '~/Desktop', icon: 'folder', score: 16 },
    { id: 'builtin:quit', kind: 'builtin', builtin: 'quit', title: 'Quit Nevermind', subtitle: 'Close the app', icon: 'power', score: 15 },
  ]
}

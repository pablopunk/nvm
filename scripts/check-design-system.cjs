const fs = require('fs')

function read(path) {
  return fs.readFileSync(path, 'utf8')
}

function assertIncludes(path, text) {
  const content = read(path)
  if (!content.includes(text)) throw new Error(`${path} must include ${text}`)
}

assertIncludes('src/command-list.tsx', 'RootCommandList')
assertIncludes('src/App.tsx', 'presentation === \'root\'')
assertIncludes('electron/main.cjs', "presentation: 'root'")
assertIncludes('electron/main.cjs', 'function clipboardHistoryView()')
assertIncludes('src/App.tsx', 'function renderShortcutManager()')
assertIncludes('src/ui.tsx', 'selectedOnlyShortcut')

console.log('design-system checks passed')

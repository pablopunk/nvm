const fs = require('fs')

function read(path) {
  return fs.readFileSync(path, 'utf8')
}

function assertIncludes(path, text) {
  const content = read(path)
  if (!content.includes(text)) throw new Error(`${path} must include ${text}`)
}

assertIncludes('src/command-list.tsx', 'RootCommandList')
assertIncludes('src/extension-view.tsx', 'presentation === \'root\'')
assertIncludes('electron/main.cjs', "presentation: 'root'")
assertIncludes('electron/main.cjs', 'function clipboardHistoryView()')
assertIncludes('src/shortcut-manager.tsx', 'ShortcutManagerView')
assertIncludes('src/use-ai-chat.ts', 'useAiChat')
assertIncludes('src/use-extension-navigation.ts', 'useExtensionNavigation')
assertIncludes('src/filtering.ts', 'filterCommandItems')
assertIncludes('src/command-icons.tsx', 'iconForAction')
assertIncludes('src/ui.tsx', 'selectedOnlyShortcut')
assertIncludes('electron/main.cjs', 'case \'nativeAction\'')
assertIncludes('electron/main.cjs', 'declaredGlobalShortcuts')

console.log('design-system checks passed')

'use strict';

const fs = require('node:fs');

function read(path) {
  return fs.readFileSync(path, 'utf8');
}

function assertIncludes(path, text) {
  const content = read(path);
  if (!content.includes(text)) {
    throw new Error(`${path} must include ${text}`);
  }
}

assertIncludes('src/command-list.tsx', 'RootCommandList');
assertIncludes('src/extension-view.tsx', "presentation === 'root'");
assertIncludes('src/electron/main.ts', "presentation: 'root'");
assertIncludes('src/electron/main.ts', 'function clipboardHistoryView()');
assertIncludes('src/shortcut-manager.tsx', 'ShortcutManagerView');
assertIncludes('src/use-ai-chat.ts', 'useAiChat');
assertIncludes('src/use-extension-navigation.ts', 'useExtensionNavigation');
assertIncludes('src/filtering.ts', 'filterCommandItems');
assertIncludes('src/command-icons.tsx', 'iconForAction');
assertIncludes('src/ui.tsx', 'selectedOnlyShortcut');
assertIncludes('src/electron/main.ts', "case 'nativeAction'");
assertIncludes('src/electron/main.ts', 'declaredGlobalShortcuts');

// biome-ignore lint/suspicious/noConsole: CLI script
console.log('design-system checks passed');

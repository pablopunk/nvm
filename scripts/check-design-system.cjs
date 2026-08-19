const fs = require('fs');

function read(path) {
  return fs.readFileSync(path, 'utf8');
}

function assertIncludes(path, text) {
  const content = read(path);
  if (!content.includes(text)) throw new Error(`${path} must include ${text}`);
}

assertIncludes('src/app/palette/command-list.tsx', 'RootCommandList');
assertIncludes('src/app/palette/extension-view.tsx', "presentation === 'root'");
assertIncludes('src/app/electron/main.ts', "presentation: 'root'");
assertIncludes('src/app/electron/main.ts', 'function clipboardHistoryView()');
assertIncludes('src/app/palette/shortcut-manager.tsx', 'ShortcutManagerView');
assertIncludes('src/app/palette/use-ai-chat.ts', 'useAiChat');
assertIncludes(
  'src/app/palette/use-extension-navigation.ts',
  'useExtensionNavigation',
);
assertIncludes('src/app/palette/filtering.ts', 'filterCommandItems');
assertIncludes('src/app/palette/command-icons.tsx', 'iconForAction');
assertIncludes('src/app/palette/ui.tsx', 'selectedOnlyShortcut');
assertIncludes('src/app/electron/main.ts', "case 'nativeAction'");
assertIncludes('src/app/electron/main.ts', 'declaredGlobalShortcuts');

console.log('design-system checks passed');

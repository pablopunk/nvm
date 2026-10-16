const fs = require('fs');
const { readElectronSources } = require('./lib/read-electron-sources.cjs');

const electronSources = readElectronSources();

function read(path) {
  return fs.readFileSync(path, 'utf8');
}

function assertElectronIncludes(text) {
  if (!electronSources.includes(text))
    throw new Error(`src/app/electron must include ${text}`);
}

function assertIncludes(path, text) {
  const content = read(path);
  if (!content.includes(text)) throw new Error(`${path} must include ${text}`);
}

assertIncludes('src/app/palette/command-list.tsx', 'RootCommandList');
assertIncludes(
  'src/app/palette/extension-view/extension-view.tsx',
  "presentation === 'root'",
);
assertElectronIncludes("presentation: 'root'");
assertElectronIncludes('function clipboardHistoryView()');
assertIncludes('src/app/palette/shortcut-manager.tsx', 'ShortcutManagerView');
assertIncludes('src/app/palette/ai-chat/use-ai-chat.ts', 'useAiChat');
assertIncludes(
  'src/app/palette/extension-view/use-extension-navigation.ts',
  'useExtensionNavigation',
);
assertIncludes('src/app/palette/filtering.ts', 'filterCommandItems');
assertIncludes('src/app/palette/command-icons.tsx', 'iconForAction');
assertIncludes('src/app/palette/ui.tsx', 'selectedOnlyShortcut');
assertElectronIncludes("case 'nativeAction'");
assertElectronIncludes('declaredGlobalShortcuts');

console.log('design-system checks passed');

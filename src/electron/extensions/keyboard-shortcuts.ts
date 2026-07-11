import { extensionContext } from './_context';

function keyboardShortcutItem(record: any) {
  const changeAction = extensionContext.buildRecordShortcutAction(
    { actionId: record.actionId, title: 'Change shortcut' },
    {},
  );
  const removeAction =
    record.source === 'user'
      ? extensionContext.buildRemoveShortcutAction(
          { actionId: record.actionId, title: 'Remove shortcut' },
          {},
        )
      : null;
  return {
    id: `shortcut:${record.actionId}`,
    title: record.title,
    subtitle: record.subtitle,
    shortcut: record.accelerator,
    icon: 'keyboard',
    primaryAction: changeAction,
    actionPanel: {
      sections: [{ actions: [changeAction, removeAction].filter(Boolean) }],
    },
  };
}

export function createKeyboardShortcutsExtension() {
  return {
    id: 'nevermind.shortcuts',
    title: 'Keyboard Shortcuts',
    permissions: ['shortcuts'] as const,
    commands: [
      {
        id: 'keyboard-shortcuts',
        actionId: 'keyboard-shortcuts',
        title: 'Keyboard Shortcuts',
        subtitle: 'View, change, or remove global shortcuts',
        icon: 'keyboard',
        score: 16,
        run: (ctx) =>
          ctx.ui.list({
            id: 'keyboard-shortcuts',
            title: 'Keyboard Shortcuts',
            presentation: 'root',
            searchBarPlaceholder: 'Search Keyboard Shortcuts',
            emptyView: { title: 'No shortcuts found.' },
            items: ctx.shortcuts.list().map(keyboardShortcutItem),
          }),
      },
    ],
  };
}

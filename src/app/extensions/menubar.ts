import { appMenuBarGlyph } from '../shared/menubar-icons';
import { extensionContext } from './_context';
import { appSettingsView } from './settings';

function updateRow() {
  const prompt =
    extensionContext.compatibilityPromptAction() ||
    extensionContext.updatePromptAction();
  if (!prompt) return null;
  return {
    id: 'update',
    title: prompt.title,
    icon: 'download' as const,
    ...(prompt.primaryAction
      ? { action: prompt.primaryAction }
      : { disabled: true }),
  };
}

function nevermindMenuBar(ctx: any) {
  const update = updateRow();
  return {
    icon: appMenuBarGlyph(ctx.settings.get('menuBarIconStyle')),
    tooltip: 'Nevermind',
    sections: [
      {
        items: [
          {
            id: 'open',
            title: 'Open Nevermind',
            shortcut: extensionContext.getPaletteHotkey() || undefined,
            action: ctx.actions.openPalette(),
          },
        ],
      },
      {
        items: [
          update || {
            id: 'check-updates',
            title: 'Check for Updates…',
            action: ctx.actions.updates.check('Check for Updates'),
          },
          {
            id: 'settings',
            title: 'Settings…',
            action: ctx.actions.run('Settings', appSettingsView),
          },
        ],
      },
      {
        items: [
          {
            id: 'quit',
            title: 'Quit Nevermind',
            action: ctx.actions.system.quit(),
          },
        ],
      },
    ],
  };
}

export function createNevermindMenuBarExtension() {
  return {
    id: 'nevermind.menubar',
    title: 'Nevermind',
    capabilities: ['updates', 'system'] as const,
    menuBar: { render: nevermindMenuBar },
  };
}

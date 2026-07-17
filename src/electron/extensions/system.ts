import os from 'node:os';
import path from 'node:path';

import { settingsTitle } from '../os';
import { systemSettingsEntries } from '../system-settings';
import { extensionContext } from './_context';

function systemItems(ctx) {
  const system = ctx.actions.system;
  return [
    {
      id: 'builtin:lock-screen',
      title: 'Lock Screen',
      subtitle: 'Secure this computer',
      icon: 'lock',
      score: 22,
      dismissAfterRun: 'auto',
      primaryAction: system.lockScreen('Lock Screen'),
    },
    {
      id: 'builtin:sleep',
      title: 'Sleep',
      subtitle: 'Put this computer to sleep',
      icon: 'moon',
      score: 21,
      dismissAfterRun: 'auto',
      primaryAction: system.sleep('Sleep'),
    },
    {
      id: 'builtin:restart',
      title: 'Restart Computer',
      subtitle: 'Restart this computer',
      icon: 'restart',
      score: 20,
      dismissAfterRun: 'auto',
      primaryAction: system.restart('Restart Computer'),
    },
    {
      id: 'builtin:settings',
      title: settingsTitle(),
      subtitle: 'Open system preferences',
      icon: 'settings',
      score: 19,
      dismissAfterRun: 'auto',
      primaryAction: system.openSystemSettings(settingsTitle()),
    },
    {
      id: 'builtin:quit',
      title: 'Quit Nevermind',
      subtitle: 'Close the app',
      icon: 'power',
      score: 15,
      dismissAfterRun: 'auto',
      appearance: { foreground: 'red' },
      primaryAction: system.quit('Quit Nevermind'),
    },
  ];
}

async function systemSettingsSearchItems(ctx, query) {
  const items = (await systemSettingsEntries()).map((entry) => ({
    id: `system-settings:${entry.id}`,
    title: entry.title,
    subtitle: 'System Settings',
    aliases: entry.aliases,
    icon: 'settings',
    score: 18,
    dismissAfterRun: 'auto',
    primaryAction: ctx.actions.system.openSystemSettings(entry.title, {
      paneId: entry.id,
    }),
  }));
  return items.filter((item) => extensionContext.rankAction(item, query));
}

function createSystemExtension() {
  const extension = {
    id: 'nevermind.system',
    title: 'System',
    permissions: ['system'] as const,
  };
  const commands = systemItems(
    extensionContext.createExtensionContext(extension, null),
  ).map(extensionContext.commandFromItem);
  return {
    ...extension,
    commands,
    rootItems: (ctx) => systemItems(ctx),
    searchItems: systemSettingsSearchItems,
  };
}

function createPlacesExtension() {
  return {
    id: 'nevermind.places',
    title: 'Places',
    permissions: ['places'] as const,
    commands: placesItems().map(extensionContext.commandFromItem),
    rootItems: () => placesItems(),
  };
}

function placesItems() {
  return [
    {
      id: 'places:downloads',
      title: 'Open Downloads',
      subtitle: '~/Downloads',
      icon: 'folder',
      score: 18,
      dismissAfterRun: 'auto',
      primaryAction: {
        type: 'openPath',
        title: 'Open Downloads',
        path: path.join(os.homedir(), 'Downloads'),
        dismissAfterRun: 'auto',
      },
    },
    {
      id: 'places:documents',
      title: 'Open Documents',
      subtitle: '~/Documents',
      icon: 'folder',
      score: 17,
      dismissAfterRun: 'auto',
      primaryAction: {
        type: 'openPath',
        title: 'Open Documents',
        path: path.join(os.homedir(), 'Documents'),
        dismissAfterRun: 'auto',
      },
    },
    {
      id: 'places:desktop',
      title: 'Open Desktop',
      subtitle: '~/Desktop',
      icon: 'folder',
      score: 16,
      dismissAfterRun: 'auto',
      primaryAction: {
        type: 'openPath',
        title: 'Open Desktop',
        path: path.join(os.homedir(), 'Desktop'),
        dismissAfterRun: 'auto',
      },
    },
  ];
}

export { createPlacesExtension, createSystemExtension };

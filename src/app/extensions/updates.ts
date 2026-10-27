import { app } from 'electron';
import { extensionContext } from './_context';

export function createUpdatesExtension() {
  const extension = {
    id: 'nevermind.updates',
    title: 'Updates',
    capabilities: ['updates'] as const,
  };
  const ctx: any = extensionContext.createExtensionContext(extension, null);
  const checkItem = () => ({
    id: 'updates:check',
    title: 'Check for Updates',
    subtitle: `Current version: ${app.getVersion()}`,
    icon: 'restart',
    score: 23,
    primaryAction: ctx.actions.updates.check('Check for Updates'),
  });
  return {
    ...extension,
    commands: [
      { ...checkItem(), run: () => extensionContext.checkForUpdatesView() },
    ],
    rootItems() {
      return [
        extensionContext.compatibilityPromptAction() ||
          extensionContext.updatePromptAction() ||
          checkItem(),
      ];
    },
  };
}

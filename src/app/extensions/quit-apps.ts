import type { NevermindExtension } from '../resources/nevermind-extension-api';

export function createQuitAppsExtension() {
  return {
    id: 'quit-apps',
    title: 'Quit Apps',
    capabilities: ['system'],
    actions(ctx) {
      return [
        {
          id: 'quit-all-apps-action',
          title: 'Quit All Apps',
          subtitle: 'Closes all visible GUI applications',
          icon: 'power-off',
          appearance: { foreground: 'red' },
          run: async (ctx) => {
            return ctx.ui.confirm({
              title: 'Quit All Apps?',
              message:
                'This will close all visible applications. Unsaved changes may be lost.',
              confirmLabel: 'Quit All',
              destructive: true,
              onConfirm: ctx.actions.run('Quitting Apps', async (ctx) => {
                const script = `
                  set quitList to {}
                  tell application "System Events"
                    set appProcesses to every process whose background only is false
                    repeat with p in appProcesses
                      set nm to name of p
                      if nm is not "Nevermind" and nm is not "Finder" then
                        copy nm to end of quitList
                      end if
                    end repeat
                  end tell

                  repeat with appName in quitList
                    try
                      tell application appName to quit
                    end try
                  end repeat
                `;

                try {
                  ctx.logs.info('Attempting to quit applications', { script });
                  await ctx.desktop.shell?.appleScript(script);
                  return ctx.ui.toast({ message: 'Quitting applications...' });
                } catch (err: any) {
                  ctx.logs.error('Failed to execute Quit Apps script', {
                    error: err.message,
                  });
                  return ctx.ui.toast({
                    message: `Error: ${err.message}`,
                    tone: 'error',
                  });
                }
              }),
            });
          },
        },
      ];
    },
  } satisfies NevermindExtension;
}

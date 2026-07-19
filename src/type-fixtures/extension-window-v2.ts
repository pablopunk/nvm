import type {
  ExtensionAction,
  ExtensionContext,
  ExtensionDraftResolution,
  ExtensionWindowRestoreDescriptor,
  NevermindExtension,
} from '../resources/nevermind-extension-api';

const view = {
  id: 'note',
  title: 'Note',
  type: 'editor',
  draft: { key: 'note:primary', version: 1, autosave: { debounceMs: 500 } },
} as const;

const migratedDraft: ExtensionDraftResolution = {
  type: 'draftResolution',
  key: 'note:primary',
  resolution: 'migrate',
  content: 'migrated',
};

function everyReleasedExtensionWindowCall(
  ctx: ExtensionContext,
): ExtensionAction[] {
  return [
    ctx.windows.create(view),
    ctx.windows.create(view, { id: 'create-id', persistent: true }),
    ctx.windows.show('note'),
    ctx.windows.show('note', 'Show Note'),
    ctx.windows.show('note', 'Show Note', { shortcut: 'Command+1' }),
    ctx.windows.hide('note'),
    ctx.windows.hide('note', 'Hide Note'),
    ctx.windows.hide('note', 'Hide Note', { shortcut: 'Command+2' }),
    ctx.windows.close('note'),
    ctx.windows.close('note', 'Close Note'),
    ctx.windows.close('note', 'Close Note', { shortcut: 'Command+3' }),
    ctx.windows.toggle('note'),
    ctx.windows.toggle('note', 'Toggle Note'),
    ctx.windows.toggle('note', { width: 640 }),
    ctx.windows.toggle('note', 'Toggle Note', { width: 640 }),
    ctx.windows.toggle(view),
    ctx.windows.toggle(view, 'Toggle Note'),
    ctx.windows.toggle(view, { id: 'toggle-id', height: 480 }),
    ctx.windows.toggle(view, 'Toggle Note', {
      id: 'toggle-id',
      height: 480,
    }),
  ];
}

const extension = {
  id: 'window-contract-fixture',
  title: 'Window Contract Fixture',
  restoreWindow(
    _ctx: ExtensionContext,
    restoreKey: string,
  ): ExtensionWindowRestoreDescriptor | null {
    if (restoreKey !== 'note') {
      return null;
    }
    return {
      view,
      options: {
        id: 'note',
        restoreKey,
        persistent: true,
        remembersFrame: true,
      },
    };
  },
  commands: [
    {
      id: 'open-note',
      title: 'Open Note',
      run(ctx: ExtensionContext) {
        const capabilities = [
          'windows.always-on-top',
          'windows.all-spaces',
          'windows.frame-restore',
          'windows.display-recovery',
        ] as const;
        for (const capability of capabilities) {
          ctx.system.capabilities.has(capability);
        }
        return ctx.windows.create(view, {
          id: 'note',
          restoreKey: 'note',
          persistent: true,
        });
      },
    },
  ],
} satisfies NevermindExtension;

export { everyReleasedExtensionWindowCall, extension, migratedDraft };

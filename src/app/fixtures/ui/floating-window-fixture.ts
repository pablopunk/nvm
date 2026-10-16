// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import { showExtensionFeedback } from '../../extensions/feedback.ts';
import type {
  ExtensionAction,
  ExtensionContext,
} from '../../resources/nevermind-extension-api';

const FLOATING_WINDOW_ID = 'dev-ui-floating-window';
const FLOATING_WINDOW_OPTIONS = {
  id: FLOATING_WINDOW_ID,
  title: 'Nevermind Floating Note',
  titleBar: 'hidden' as const,
  chrome: 'default' as const,
  width: 560,
  height: 520,
  alwaysOnTop: true,
  visibleOnAllSpaces: true,
};

function floatingNoteEditorView(ctx: ExtensionContext) {
  const copyNote = ctx.actions.copyText('Floating note', 'Copy Note');
  const prompt = ctx.input.prompt({
    title: 'Label Floating Note',
    message: 'Prompts expand from the compact action panel.',
    fields: [
      {
        id: 'label',
        label: 'Label',
        type: 'text',
        placeholder: 'Scratchpad',
        required: true,
      },
    ],
    submitTitle: 'Apply Label',
    action: ctx.actions.run('Apply Label', (_ctx, action) =>
      showExtensionFeedback(
        _ctx,
        'Floating Note',
        `Label: ${String(action.formValues?.label || '')}`,
      ),
    ),
  });
  const confirm = ctx.ui.confirm({
    title: 'Confirm Floating Action',
    message: 'Confirmations stay in the compact action panel.',
    confirmLabel: 'Confirm',
    onConfirm: ctx.actions.run('Confirm', (_ctx) =>
      showExtensionFeedback(_ctx, 'Floating Note', 'Floating action confirmed'),
    ),
  });
  const submenu: ExtensionAction = {
    type: 'submenu',
    title: 'More Actions',
    icon: 'ellipsis',
    submenu: {
      title: 'More Actions',
      sections: [{ actions: [copyNote] }],
    },
  };
  return ctx.ui.editor({
    id: FLOATING_WINDOW_ID,
    title: 'Floating Note',
    subtitle: 'Editable host-rendered note',
    format: 'markdown',
    titleFromContent: true,
    placeholder: 'Write a floating note…',
    content:
      '# Floating Note\n\nEdit this note in a real independent window.\n\n- Always on top\n- Reuses ctx.ui.editor(...)\n- Compact Cmd+K action panel by default',
    actions: [copyNote, prompt, confirm, submenu],
  });
}

export function floatingWindowToggleAction(ctx: ExtensionContext) {
  return ctx.action({
    id: 'toggle-floating-note',
    title: 'Toggle Floating Note',
    subtitle:
      'Persistent action: creates the note if needed, then shows or hides it',
    icon: 'panel-top-open',
    keywords: ['window', 'note', 'shortcut', 'persistent action'],
    action: ctx.windows.toggle(
      floatingNoteEditorView(ctx),
      FLOATING_WINDOW_OPTIONS,
    ),
  });
}

function floatingWindowToggleItem(ctx: ExtensionContext) {
  return ctx.ui.item({
    id: 'toggle-floating-note',
    title: 'Toggle Floating Note',
    subtitle:
      'References the persistent action, so aliases and shortcuts attach to the durable action id',
    icon: 'panel-top-open',
    keywords: ['window', 'note', 'shortcut', 'persistent action'],
    primaryAction: ctx.actions.ref(
      'toggle-floating-note',
      'Toggle Floating Note',
    ),
  });
}

export function floatingWindowView(ctx: ExtensionContext) {
  return ctx.ui.list({
    id: 'dev-ui-floating-window-controls',
    title: 'Dev UI · Floating Window',
    subtitle:
      'Open, toggle, and close an independent host-rendered extension window',
    items: [
      floatingWindowToggleItem(ctx),
      ctx.ui.item({
        id: 'close',
        title: 'Close Floating Note',
        subtitle: 'Closes the existing window',
        icon: 'x',
        primaryAction: ctx.windows.close(
          FLOATING_WINDOW_ID,
          'Close Floating Note',
        ),
      }),
    ],
  });
}

// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

export function textInputView(ctx: ExtensionContext) {
  return ctx.ui.list({
    id: 'dev-ui-text-input',
    title: 'Dev UI · Text Input',
    subtitle: 'Paste/type primitives for snippet and selected-text workflows',
    items: [
      ctx.ui.item({
        id: 'paste-concealed-restore',
        title: 'Paste concealed text and restore clipboard',
        subtitle:
          'Paste into the frontmost app without keeping this text in clipboard history',
        icon: 'clipboard-paste',
        primaryAction: ctx.actions.pasteText(
          'Nevermind concealed restored paste',
          'Paste Concealed + Restore',
          { restoreClipboard: true, concealed: true, dismissAfterRun: 'auto' },
        ),
        actions: [
          ctx.actions.pasteText(
            'Nevermind concealed restored paste',
            'Paste Concealed + Restore',
            {
              restoreClipboard: true,
              concealed: true,
              dismissAfterRun: 'auto',
            },
          ),
        ],
      }),
      ctx.ui.item({
        id: 'paste-keep-open',
        title: 'Paste text and keep palette open',
        subtitle: 'Useful for repeated snippet insertion tests',
        icon: 'panel-top-open',
        primaryAction: ctx.actions.pasteText(
          'Nevermind keep-open paste',
          'Paste and Keep Open',
          { keepPaletteOpen: true, dismissAfterRun: 'auto' },
        ),
        actions: [
          ctx.actions.pasteText(
            'Nevermind keep-open paste',
            'Paste and Keep Open',
            { keepPaletteOpen: true, dismissAfterRun: 'auto' },
          ),
        ],
      }),
      ctx.ui.item({
        id: 'type-text',
        title: 'Type text without clipboard',
        subtitle: 'Uses host keyboard typing when supported',
        icon: 'keyboard',
        primaryAction: ctx.actions.typeText(
          'Nevermind typed text',
          'Type Text',
          { dismissAfterRun: 'auto' },
        ),
        actions: [
          ctx.actions.typeText('Nevermind typed text', 'Type Text', {
            dismissAfterRun: 'auto',
          }),
        ],
      }),
    ],
  });
}

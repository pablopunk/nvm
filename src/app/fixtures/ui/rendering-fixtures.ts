// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import type { ExtensionContext } from '../../resources/nevermind-extension-api';

export function renderingPolishView(ctx: ExtensionContext) {
  const openAction = ctx.actions.openUrl('https://nvm.fyi', 'Open Link');
  return ctx.ui.list({
    id: 'dev-ui-rendering-polish',
    title: 'Dev UI · Rendering Polish',
    subtitle:
      'Accessories, metadata details, image descriptors, and side inspector panes',
    selectedItemId: 'details',
    detail: { visible: true, placement: 'side' },
    items: [
      ctx.ui.item({
        id: 'details',
        title: 'Detailed Result',
        subtitle: 'Selected row renders an inspector pane',
        icon: 'sparkles',
        accessories: [
          { text: 'Ready', tone: 'success' },
          { text: 'AI', tone: 'accent', tooltip: 'Generated-friendly host UI' },
        ],
        image: {
          light:
            'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"%3E%3Crect width="64" height="64" rx="16" fill="%23f5f1e8"/%3E%3Cpath d="M18 40L29 18l6 16 11-10-10 24-7-16z" fill="%23906f00"/%3E%3C/svg%3E',
          dark: 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"%3E%3Crect width="64" height="64" rx="16" fill="%23202232"/%3E%3Cpath d="M18 40L29 18l6 16 11-10-10 24-7-16z" fill="%23ffd84d"/%3E%3C/svg%3E',
          shape: 'rounded',
          alt: 'Nevermind fixture mark',
        },
        detail: {
          title: 'Detailed Result',
          subtitle: 'Host-rendered metadata and markdown',
          markdown:
            'Use `detail.markdown`, `detail.metadata`, toned accessories, and detail actions instead of jumping to a webview for common inspector layouts.',
          metadata: [
            { label: 'Status', value: 'Ready', type: 'tag', tone: 'success' },
            { label: 'Kind', value: 'Fixture' },
            {
              type: 'link',
              label: 'Docs',
              value: 'nvm.fyi',
              url: 'https://nvm.fyi',
            },
            { type: 'separator' },
            { label: 'Updated', value: new Date().toLocaleString() },
          ],
          actions: [openAction],
        },
        primaryAction: openAction,
        actions: [openAction],
      }),
      ctx.ui.item({
        id: 'accent-background',
        title: 'Accent Background',
        subtitle: 'Highlights a result without replacing selection',
        icon: 'sparkles',
        appearance: { foreground: 'yellow', background: 'accent' },
        detail: {
          title: 'Accent Background',
          markdown:
            '`appearance.background: "accent"` adds a subtle semantic highlight while the selected state remains visible.',
        },
      }),
      ctx.ui.item({
        id: 'primary-action',
        title: 'Copy Fixture Label',
        subtitle: 'Primary action with host-owned elevation and selection',
        icon: 'copy',
        appearance: { prominence: 'primary' },
        primaryAction: ctx.actions.copyText(
          'Primary action',
          'Copy Fixture Label',
        ),
      }),
      ctx.ui.item({
        id: 'warning',
        title: 'Warning Accessory',
        subtitle: 'Foreground colors title and icon together',
        icon: 'alert-triangle',
        appearance: { foreground: 'orange' },
        accessories: [
          { text: 'Warning', tone: 'warning' },
          { text: 'Muted', tone: 'muted' },
        ],
        detail: {
          title: 'Warning Accessory',
          markdown:
            '`appearance.foreground` colors both the row title and the Lucide icon while accessories keep their own tones.',
          metadata: [
            { type: 'tag', label: 'Tone', value: 'warning', tone: 'warning' },
          ],
        },
      }),
      ctx.ui.item({
        id: 'danger',
        title: 'Danger Accessory',
        subtitle: 'Danger foreground matches icon and title',
        icon: 'shield-alert',
        appearance: { foreground: 'red' },
        accessories: [{ text: 'Danger', tone: 'danger' }],
        detail: {
          title: 'Danger Accessory',
          markdown:
            'Foreground styling uses existing semantic color tokens and remains compact.',
        },
      }),
    ],
  });
}

export function sidePreviewView(ctx: ExtensionContext) {
  return ctx.ui.list({
    id: 'dev-ui-side-preview',
    title: 'Dev UI · Side Preview',
    subtitle: 'Select an item to preview it beside the list.',
    presentation: 'side-preview',
    selectedItemId: 'text',
    items: [
      ctx.ui.item({
        id: 'text',
        title: 'Text Preview',
        subtitle: 'Plain text renders in the side pane',
        icon: 'clipboard',
        primaryAction: ctx.ui.confirm({
          title: 'Confirm side preview action',
          message: 'Confirmation rows should remain compact.',
          confirmLabel: 'Confirm',
          onConfirm: ctx.actions.run('Confirm', () => {}),
        }),
        detail: {
          title: 'Text Preview',
          subtitle: 'Side preview fixture',
          text: 'This is a text preview rendered beside the selectable list.',
        },
      }),
      ctx.ui.item({
        id: 'markdown',
        title: 'Markdown Preview',
        subtitle: 'Existing detail markdown remains supported',
        icon: 'file-text',
        detail: {
          title: 'Markdown Preview',
          markdown: 'Side previews can also reuse **host-rendered Markdown**.',
        },
      }),
    ],
  });
}

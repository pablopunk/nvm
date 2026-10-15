// biome-ignore-all lint: Dev-only UI fixtures intentionally exercise dynamic extension payload shapes.
import { showExtensionFeedback } from '../../extensions/feedback.ts';
import type { ExtensionContext } from '../../resources/nevermind-extension-api';
import {
  fileIndexControlsAction,
  openWithAppsAction,
  reindexDownloadsAction,
} from './file-index-fixtures.ts';
import { floatingWindowView } from './floating-window-fixture.ts';
import { formView } from './form-fixture.ts';
import { ocrFixtureView } from './ocr-fixture.ts';
import { previewView } from './preview-fixture.ts';
import { editorView, promptView } from './prompt-editor-fixtures.ts';
import { textInputView } from './text-input-fixture.ts';

export function listView(ctx: ExtensionContext) {
  const confirm = ctx.ui.confirm({
    title: 'Confirm Dev Action',
    message: 'This confirms host-owned action UI still renders correctly.',
    confirmLabel: 'Show Indicator',
    onConfirm: ctx.actions.run('Show Indicator', (innerCtx) =>
      showExtensionFeedback(
        innerCtx,
        'List Fixture',
        'Confirmed from dev UI fixture',
      ),
    ),
  });
  return ctx.ui.list({
    id: 'dev-ui-list',
    title: 'Dev UI · List',
    subtitle:
      'List rows, accessories, sections, action panels, confirmation, and navigation',
    searchBarPlaceholder: 'Filter fixture rows',
    sections: [
      {
        title: 'Rows',
        items: [
          ctx.ui.item({
            id: 'form',
            title: 'Open Form Fixture',
            subtitle: 'Textarea, dropdowns, errors, descriptions',
            icon: 'list-checks',
            accessories: [{ text: 'form' }],
            primaryAction: ctx.actions.push('Open Form', formView(ctx)),
          }),
          ctx.ui.item({
            id: 'root-navigation',
            title: 'Open Root Navigation Fixture',
            subtitle: 'Replaces the stack; Escape returns to the palette root',
            icon: 'corner-up-left',
            accessories: [{ text: 'root' }],
            primaryAction: ctx.actions.root(
              'Open Root Navigation',
              ctx.ui.preview({
                id: 'dev-ui-root-navigation',
                title: 'Dev UI · Root Navigation',
                content:
                  '# Root Navigation\n\nThis view intentionally resets the palette navigation stack.',
              }),
            ),
          }),
          ctx.ui.item({
            id: 'text-input',
            title: 'Open Text Input Fixture',
            subtitle: 'Paste/type actions for snippets and transforms',
            icon: 'keyboard',
            accessories: [{ text: 'text' }],
            primaryAction: ctx.actions.push(
              'Open Text Input',
              textInputView(ctx),
            ),
          }),
          ctx.ui.item({
            id: 'floating-window',
            title: 'Open Floating Window Fixture',
            subtitle: 'Independent host-rendered extension window',
            icon: 'panel-top-open',
            accessories: [{ text: 'window' }],
            primaryAction: ctx.actions.push(
              'Open Floating Window',
              floatingWindowView(ctx),
            ),
          }),
          ctx.ui.item({
            id: 'prompt',
            title: 'Open Prompt Fixture',
            subtitle: 'Prompted arguments before an action runs',
            icon: 'text-cursor-input',
            accessories: [{ text: 'prompt' }],
            primaryAction: ctx.actions.push('Open Prompt', promptView(ctx)),
          }),
          ctx.ui.item({
            id: 'editor',
            title: 'Open Editor Fixture',
            subtitle: 'Editable markdown, preview, submit payload',
            icon: 'file-pen-line',
            accessories: [{ text: 'editor' }],
            primaryAction: ctx.actions.push('Open Editor', editorView(ctx)),
          }),
          ctx.ui.item({
            id: 'preview',
            title: 'Open Preview Fixture',
            subtitle: 'Markdown/text preview',
            icon: 'file-text',
            accessories: [{ text: 'preview' }],
            primaryAction: ctx.actions.push('Open Preview', previewView(ctx)),
          }),
          ctx.ui.item({
            id: 'file-index',
            title: 'File Index Controls',
            subtitle:
              'Snapshot and bounded reindex helpers for generated file searchers',
            icon: 'folder-search',
            accessories: [{ text: 'files' }],
            primaryAction: fileIndexControlsAction(ctx),
            actions: [
              fileIndexControlsAction(ctx),
              reindexDownloadsAction(ctx),
              openWithAppsAction(ctx),
            ],
          }),
          ctx.ui.item({
            id: 'ocr',
            title: 'OCR Fixture',
            subtitle: 'Image, screen, and region text recognition helpers',
            icon: 'scan-text',
            accessories: [{ text: 'ocr' }],
            primaryAction: ctx.actions.push('Open OCR', ocrFixtureView(ctx)),
          }),
          ctx.ui.item({
            id: 'confirm',
            title: 'Confirmation Fixture',
            subtitle: 'Host-owned confirm step',
            icon: 'shield-check',
            accessories: [{ text: 'confirm' }],
            primaryAction: confirm,
            actionPanel: {
              sections: [
                {
                  actions: [
                    confirm,
                    ctx.actions.copyText(
                      'copied from dev fixture',
                      'Copy Fixture Text',
                    ),
                  ],
                },
              ],
            },
          }),
        ],
      },
    ],
  });
}

export function loadingView(ctx: ExtensionContext) {
  return ctx.ui.list({
    id: 'dev-ui-loading',
    title: 'Dev UI · Loading',
    contentSizing: 'fit',
    subtitle: 'One status item with a content-fit loading perimeter.',
    isLoading: true,
    items: [
      {
        id: 'loading',
        title: 'Loading fixture item…',
        subtitle: 'The list wraps this row instead of filling the palette.',
        icon: 'loader-circle',
      },
    ],
  });
}

export function emptyListView(ctx: ExtensionContext) {
  return ctx.ui.list({
    id: 'dev-ui-empty-list',
    title: 'Dev UI · Empty List',
    subtitle: 'A deterministic empty list for reviewing the empty state.',
    emptyView: {
      title: 'No items',
      subtitle: 'This list intentionally has no items.',
    },
    items: [],
  });
}
